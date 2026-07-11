package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/offsite"
	"github.com/boomerang-backup/boomerang/internal/pathutil"
)

type restoreR2Req struct {
	AccountID string `json:"accountId"`
	Bucket    string `json:"bucket"`
	Prefix    string `json:"prefix"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

func (s *Server) handleTestRestoreR2(w http.ResponseWriter, r *http.Request) {
	if err := s.guardFreshInstall(); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	if !s.allowSetup(s.clientIP(r)) {
		writeErr(w, http.StatusTooManyRequests, "too many setup attempts")
		return
	}
	cfg, err := restoreConfigFromBody(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := offsite.TestConnection(cfg); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRestoreFromR2(w http.ResponseWriter, r *http.Request) {
	if err := s.guardFreshInstall(); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	if !s.allowSetup(s.clientIP(r)) {
		writeErr(w, http.StatusTooManyRequests, "too many setup attempts")
		return
	}
	cfg, err := restoreConfigFromBody(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := offsite.TestConnection(cfg); err != nil {
		writeErr(w, http.StatusBadRequest, "bucket connection failed: "+err.Error())
		return
	}

	staging := filepath.Join(s.cfg.DataDir, ".restore-staging")
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o700); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer func() { _ = os.RemoveAll(staging) }()

	log.Printf("restoring appliance from R2 bucket %s (staging)", cfg.Bucket)
	res, err := offsite.Restore(r.Context(), staging, cfg, func(line string) {
		log.Printf("%s", line)
	})
	if err != nil {
		log.Printf("R2 restore failed: %v", err)
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	if s.sched != nil {
		s.sched.Stop()
	}
	if s.store != nil {
		_ = s.store.Close()
		s.store = nil
	}

	if err := copyTree(staging, s.cfg.DataDir); err != nil {
		log.Printf("R2 restore apply failed: %v", err)
		writeErr(w, http.StatusInternalServerError, err.Error())
		go exitSoon(1)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"files":           res.Files,
		"bytes":           res.Bytes,
		"restartRequired": true,
		"message":         "Appliance restored. The service is restarting — sign in with your previous admin password.",
	})
	go exitSoon(0)
}

func (s *Server) guardFreshInstall() error {
	if s.store == nil {
		return fmt.Errorf("database unavailable")
	}
	setup, err := s.store.IsSetup()
	if err != nil {
		return err
	}
	if setup {
		return fmt.Errorf("appliance is already set up — R2 restore is only available on first install")
	}
	return nil
}

func restoreConfigFromBody(r *http.Request) (offsite.Config, error) {
	var req restoreR2Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return offsite.Config{}, fmt.Errorf("invalid json")
	}
	cfg := offsite.Config{
		AccountID: strings.TrimSpace(req.AccountID),
		Bucket:    strings.TrimSpace(req.Bucket),
		Prefix:    strings.TrimSpace(req.Prefix),
		AccessKey: strings.TrimSpace(req.AccessKey),
		SecretKey: strings.TrimSpace(req.SecretKey),
	}
	if cfg.AccountID == "" || cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return offsite.Config{}, fmt.Errorf("account ID, bucket, access key, and secret key are required")
	}
	return cfg, nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target, err := pathutil.SafeDataPath(dst, rel)
		if err != nil {
			return fmt.Errorf("unsafe restore path %q: %w", rel, err)
		}
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func exitSoon(code int) {
	time.Sleep(800 * time.Millisecond)
	os.Exit(code)
}
