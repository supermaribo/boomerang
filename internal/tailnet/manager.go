package tailnet

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tailscale.com/ipn"
	"tailscale.com/tsnet"
)

// Status is a snapshot of Tailscale connectivity for the UI.
type Status struct {
	Running     bool     `json:"running"`
	Connected   bool     `json:"connected"`
	Hostname    string   `json:"hostname"`
	DNSName     string   `json:"dnsName,omitempty"`
	IPs         []string `json:"ips,omitempty"`
	URLs        []string `json:"urls,omitempty"`
	BackendState string  `json:"backendState,omitempty"`
	LastError   string   `json:"lastError,omitempty"`
}

// Config configures a Tailscale join.
type Config struct {
	Hostname string
	AuthKey  string
	DataDir  string // appliance data dir; state under DataDir/tailscale
}

// Manager runs an embedded tsnet node and serves an HTTP handler on the Tailnet.
type Manager struct {
	mu      sync.Mutex
	handler http.Handler
	cfg     Config
	srv     *tsnet.Server
	lnHTTP  net.Listener
	lnTLS   net.Listener
	httpSrv *http.Server
	tlsSrv  *http.Server
	running bool
	lastErr string
}

func NewManager(handler http.Handler) *Manager {
	return &Manager{handler: handler}
}

func StateDir(dataDir string) string {
	return filepath.Join(dataDir, "tailscale")
}

// Start joins the Tailnet (if needed) and serves the handler on :80 and, when
// possible, HTTPS on :443.
func (m *Manager) Start(cfg Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	hostname := strings.TrimSpace(cfg.Hostname)
	if hostname == "" {
		hostname = "boomerang"
	}
	if strings.TrimSpace(cfg.DataDir) == "" {
		return fmt.Errorf("data dir required")
	}
	dir := StateDir(cfg.DataDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("tailscale state dir: %w", err)
	}
	hasState := stateExists(dir)
	authKey := strings.TrimSpace(cfg.AuthKey)
	if !hasState && authKey == "" {
		return fmt.Errorf("auth key required for first Tailscale connect")
	}

	ts := &tsnet.Server{
		Hostname: hostname,
		Dir:      dir,
		AuthKey:  authKey,
		Logf:     func(format string, args ...any) { log.Printf("tailscale: "+format, args...) },
	}

	lnHTTP, err := ts.Listen("tcp", ":80")
	if err != nil {
		_ = ts.Close()
		m.lastErr = err.Error()
		return fmt.Errorf("tailscale listen :80: %w", err)
	}

	var lnTLS net.Listener
	if l, err := ts.ListenTLS("tcp", ":443"); err == nil {
		lnTLS = l
	} else {
		log.Printf("tailscale: HTTPS :443 unavailable (%v); HTTP :80 only", err)
	}

	httpSrv := &http.Server{Handler: m.handler}
	go func() {
		if err := httpSrv.Serve(lnHTTP); err != nil && err != http.ErrServerClosed {
			log.Printf("tailscale http serve: %v", err)
			m.mu.Lock()
			m.lastErr = err.Error()
			m.mu.Unlock()
		}
	}()

	var tlsSrv *http.Server
	if lnTLS != nil {
		tlsSrv = &http.Server{Handler: m.handler}
		go func() {
			if err := tlsSrv.Serve(lnTLS); err != nil && err != http.ErrServerClosed {
				log.Printf("tailscale https serve: %v", err)
				m.mu.Lock()
				m.lastErr = err.Error()
				m.mu.Unlock()
			}
		}()
	}

	m.cfg = cfg
	m.cfg.Hostname = hostname
	m.srv = ts
	m.lnHTTP = lnHTTP
	m.lnTLS = lnTLS
	m.httpSrv = httpSrv
	m.tlsSrv = tlsSrv
	m.running = true
	m.lastErr = ""

	// Wait briefly for backend to come up so Status is useful after Connect.
	go m.waitUp()
	return nil
}

func (m *Manager) waitUp() {
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		st := m.Status()
		if st.Connected {
			log.Printf("tailscale: connected as %s (%s)", st.DNSName, strings.Join(st.IPs, ", "))
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// Stop closes Tailnet listeners but keeps node state on disk.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked()
}

func (m *Manager) stopLocked() error {
	if !m.running {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if m.httpSrv != nil {
		_ = m.httpSrv.Shutdown(ctx)
	}
	if m.tlsSrv != nil {
		_ = m.tlsSrv.Shutdown(ctx)
	}
	if m.lnHTTP != nil {
		_ = m.lnHTTP.Close()
	}
	if m.lnTLS != nil {
		_ = m.lnTLS.Close()
	}
	var err error
	if m.srv != nil {
		err = m.srv.Close()
	}
	m.srv = nil
	m.lnHTTP = nil
	m.lnTLS = nil
	m.httpSrv = nil
	m.tlsSrv = nil
	m.running = false
	return err
}

// Forget stops, removes persisted Tailscale state, and clears last error.
func (m *Manager) Forget(dataDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_ = m.stopLocked()
	dir := StateDir(dataDir)
	if err := os.RemoveAll(dir); err != nil {
		m.lastErr = err.Error()
		return err
	}
	m.lastErr = ""
	return nil
}

// Status returns the current Tailnet status for the UI.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := Status{
		Running:   m.running,
		Hostname:  m.cfg.Hostname,
		LastError: m.lastErr,
	}
	if !m.running || m.srv == nil {
		return out
	}
	lc, err := m.srv.LocalClient()
	if err != nil {
		out.LastError = err.Error()
		return out
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	st, err := lc.Status(ctx)
	if err != nil {
		out.LastError = err.Error()
		return out
	}
	out.BackendState = st.BackendState
	out.Connected = strings.EqualFold(st.BackendState, ipn.Running.String())
	if st.Self != nil {
		if st.Self.DNSName != "" {
			out.DNSName = strings.TrimSuffix(st.Self.DNSName, ".")
		}
		for _, addr := range st.TailscaleIPs {
			out.IPs = append(out.IPs, addr.String())
		}
		if !out.Connected && st.Self.Online {
			out.Connected = true
		}
	}
	if out.DNSName != "" {
		out.URLs = append(out.URLs, "http://"+out.DNSName)
		if m.lnTLS != nil {
			out.URLs = append([]string{"https://" + out.DNSName}, out.URLs...)
		}
	}
	for _, ip := range out.IPs {
		if strings.Contains(ip, ":") {
			continue // skip IPv6 in simple URL list
		}
		out.URLs = append(out.URLs, "http://"+ip)
		if m.lnTLS != nil {
			out.URLs = append(out.URLs, "https://"+ip)
		}
	}
	return out
}

// Running reports whether the Tailnet HTTP server is active.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func stateExists(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		return true
	}
	return false
}
