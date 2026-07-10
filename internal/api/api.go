package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/boomerang-backup/boomerang/internal/config"
	"github.com/boomerang-backup/boomerang/internal/crypto"
	"github.com/boomerang-backup/boomerang/internal/jobs"
	"github.com/boomerang-backup/boomerang/internal/offsite"
	"github.com/boomerang-backup/boomerang/internal/store"
	"github.com/boomerang-backup/boomerang/internal/tzutil"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	cfg    *config.Config
	store  *store.Store
	box    *crypto.Box
	webFS  fs.FS
	runner *jobs.Runner
	sched  *jobs.Scheduler
	offsite *offsite.Syncer
	mu     sync.Mutex
	sess   map[string]session
	loginN map[string][]time.Time
}

type session struct {
	expires time.Time
	epoch   int
}

func New(cfg *config.Config, st *store.Store, box *crypto.Box, webFS fs.FS, runner *jobs.Runner) *Server {
	return &Server{
		cfg:    cfg,
		store:  st,
		box:    box,
		webFS:  webFS,
		runner: runner,
		sess:   map[string]session{},
		loginN: map[string][]time.Time{},
	}
}

func (s *Server) SetScheduler(sched *jobs.Scheduler) {
	s.sched = sched
}

func (s *Server) SetOffsite(syncer *offsite.Syncer) {
	s.offsite = syncer
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", s.handleHealth)
		r.Get("/status", s.handleStatus)
		r.Post("/setup", s.handleSetup)
		r.Post("/setup/test-r2", s.handleTestRestoreR2)
		r.Post("/setup/restore-r2", s.handleRestoreFromR2)
		r.Post("/login", s.handleLogin)
		r.Post("/logout", s.handleLogout)
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/me", s.handleMe)
			r.Get("/dashboard", s.handleDashboard)
			s.routesTargets(r)
			s.routesExtra(r)
		})
	})

	r.Handle("/*", s.spaHandler())
	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeHealth(w, false)
}

func (s *Server) writeHealth(w http.ResponseWriter, _ bool) {
	status := "ok"
	code := http.StatusOK
	checks := map[string]any{}

	if err := s.store.DB.Ping(); err != nil {
		checks["database"] = false
		status = "degraded"
		code = http.StatusServiceUnavailable
	} else {
		checks["database"] = true
	}

	if st, err := os.Stat(s.cfg.DataDir); err != nil || !st.IsDir() {
		checks["dataDir"] = false
		status = "degraded"
		code = http.StatusServiceUnavailable
	} else {
		checks["dataDir"] = true
	}

	if free, ok := diskFree(s.cfg.DataDir); ok {
		checks["diskFreeBytes"] = free
	}

	if s.runner != nil {
		running, queued := s.runner.Stats()
		checks["jobsRunning"] = running
		checks["jobsQueued"] = queued
	}

	checks["status"] = status
	writeJSON(w, code, checks)
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	setup, err := s.store.IsSetup()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"setupRequired": !setup,
		"product":       "Boomerang",
	})
}

type setupReq struct {
	Password string `json:"password"`
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	setup, err := s.store.IsSetup()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if setup {
		writeErr(w, http.StatusConflict, "already set up")
		return
	}
	var req setupReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.Password) < 8 {
		writeErr(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "hash failed")
		return
	}
	if err := s.store.SetMeta("admin_password_hash", string(hash)); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.Audit("setup", "admin password created")
	token, err := s.createSession()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "session failed")
		return
	}
	setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type loginReq struct {
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := r.RemoteAddr
	if !s.allowLogin(ip) {
		writeErr(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}
	setup, err := s.store.IsSetup()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !setup {
		writeErr(w, http.StatusBadRequest, "setup required")
		return
	}
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	hash, ok, err := s.store.GetMeta("admin_password_hash")
	if err != nil || !ok {
		writeErr(w, http.StatusInternalServerError, "no admin")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		_ = s.store.Audit("login_failed", ip)
		writeErr(w, http.StatusUnauthorized, "invalid password")
		return
	}
	token, err := s.createSession()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "session failed")
		return
	}
	_ = s.store.Audit("login", ip)
	setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("boomerang_session")
	if err == nil {
		s.mu.Lock()
		delete(s.sess, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "boomerang_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"user":     "admin",
		"product":  "Boomerang",
		"timezone": tzutil.Name(s.store),
	})
}

func (s *Server) handleDashboard(w http.ResponseWriter, _ *http.Request) {
	fsCount, _ := s.store.CountFileServers()
	dbCount, _ := s.store.CountDatabases()
	backupCount, _ := s.store.CountBackupVersions()
	storageBytes, _ := s.store.SumBackupBytes()
	recent, _ := s.store.ListRecentVersions(15, "")
	jobs, _ := s.store.ListRecentJobs(10)
	writeJSON(w, http.StatusOK, map[string]any{
		"fileServers":      fsCount,
		"databases":        dbCount,
		"backupCount":      backupCount,
		"storageBytes":     storageBytes,
		"dataDir":          s.cfg.DataDir,
		"recentBackups":    recent,
		"recentJobs":       jobs,
		"applianceStatus":  s.applianceStatus(),
	})
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("boomerang_session")
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		s.mu.Lock()
		sess, ok := s.sess[c.Value]
		epoch := s.store.SessionEpoch()
		if ok && time.Now().Before(sess.expires) && sess.epoch == epoch {
			sess.expires = time.Now().Add(24 * time.Hour)
			s.sess[c.Value] = sess
			s.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}
		if ok {
			delete(s.sess, c.Value)
		}
		s.mu.Unlock()
		writeErr(w, http.StatusUnauthorized, "unauthorized")
	})
}

func (s *Server) createSession() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	s.mu.Lock()
	s.sess[token] = session{expires: time.Now().Add(24 * time.Hour), epoch: s.store.SessionEpoch()}
	s.mu.Unlock()
	return token, nil
}

func (s *Server) allowLogin(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	cut := now.Add(-15 * time.Minute)
	var kept []time.Time
	for _, t := range s.loginN[ip] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= 20 {
		s.loginN[ip] = kept
		return false
	}
	s.loginN[ip] = append(kept, now)
	return true
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "boomerang_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
}

func (s *Server) spaHandler() http.Handler {
	if s.webFS == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, fallbackHTML)
		})
	}
	fileServer := http.FileServer(http.FS(s.webFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		f, err := s.webFS.Open(path)
		if err != nil {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		_ = f.Close()
		fileServer.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

const fallbackHTML = `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>Boomerang</title>
<style>
:root{--bg:#0f1419;--panel:#1a222c;--text:#e8eef4;--muted:#8b9aab;--accent:#3d9b8f;--line:#2a3542}
*{box-sizing:border-box}body{margin:0;font-family:ui-sans-serif,system-ui,sans-serif;background:radial-gradient(1200px 600px at 10% -10%,#1c3a36 0%,var(--bg) 55%);color:var(--text);min-height:100vh;display:grid;place-items:center}
.card{width:min(420px,92vw);background:var(--panel);border:1px solid var(--line);border-radius:16px;padding:2rem;box-shadow:0 20px 60px rgba(0,0,0,.35)}
h1{margin:0 0 .25rem;font-size:1.75rem;letter-spacing:-.02em}p{color:var(--muted);margin:0 0 1.5rem}
label{display:block;font-size:.85rem;color:var(--muted);margin-bottom:.35rem}
input{width:100%;padding:.75rem .9rem;border-radius:10px;border:1px solid var(--line);background:#12181f;color:var(--text);margin-bottom:1rem}
button{width:100%;padding:.8rem;border:0;border-radius:10px;background:var(--accent);color:#041210;font-weight:600;cursor:pointer}
.err{color:#f07178;font-size:.9rem;min-height:1.2em;margin-bottom:.75rem}
</style></head><body><div class="card" id="app"></div>
<script>
async function status(){const r=await fetch('/api/status');return r.json()}
function view(html){document.getElementById('app').innerHTML=html}
async function boot(){
  const st=await status();
  if(st.setupRequired){
    view('<h1>Boomerang</h1><p>First-run setup — choose an admin password.</p><div class="err" id="e"></div><label>Password</label><input id="p" type="password" autocomplete="new-password"/><label>Confirm</label><input id="c" type="password"/><button id="b">Create admin</button>');
    document.getElementById('b').onclick=async()=>{
      const p=document.getElementById('p').value,c=document.getElementById('c').value;
      if(p!==c){document.getElementById('e').textContent='Passwords do not match';return}
      const r=await fetch('/api/setup',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({password:p})});
      const j=await r.json(); if(!r.ok){document.getElementById('e').textContent=j.error||'failed';return} location.href='/';
    };
  } else {
    view('<h1>Boomerang</h1><p>Sign in to manage backups.</p><div class="err" id="e"></div><label>Password</label><input id="p" type="password" autocomplete="current-password"/><button id="b">Sign in</button>');
    document.getElementById('b').onclick=async()=>{
      const r=await fetch('/api/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({password:document.getElementById('p').value})});
      const j=await r.json(); if(!r.ok){document.getElementById('e').textContent=j.error||'failed';return} location.href='/app';
    };
  }
}
boot().catch(e=>view('<h1>Boomerang</h1><p class="err">'+e+'</p>'));
</script></body></html>`
