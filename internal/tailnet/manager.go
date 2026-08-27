package tailnet

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/boomerang-backup/boomerang/internal/tsdial"
)

const (
	helperPath  = "/usr/local/sbin/boomerang-tailscale"
	requestName = "tailscale-request.json"
)

// Status is a snapshot of Tailscale connectivity for the UI.
type Status struct {
	Running      bool     `json:"running"`
	Connected    bool     `json:"connected"`
	Hostname     string   `json:"hostname"`
	DNSName      string   `json:"dnsName,omitempty"`
	IPs          []string `json:"ips,omitempty"`
	URLs         []string `json:"urls,omitempty"`
	BackendState string   `json:"backendState,omitempty"`
	LastError    string   `json:"lastError,omitempty"`
	Mode         string   `json:"mode,omitempty"` // tun | userspace | none
	SocksAddr    string   `json:"socksAddr,omitempty"`
	TunAvailable bool     `json:"tunAvailable"`
}

// Config configures a Tailscale join.
type Config struct {
	Hostname string
	AuthKey  string
	DataDir  string
}

// Manager drives system Tailscale via the root helper.
type Manager struct {
	mu      sync.Mutex
	dataDir string
	lastErr string
}

func NewManager(_ any) *Manager {
	return &Manager{}
}

// StateDir is kept for API compatibility (legacy tsnet state cleanup).
func StateDir(dataDir string) string {
	return filepath.Join(dataDir, "tailscale")
}

func (m *Manager) requestPath(dataDir string) string {
	return filepath.Join(dataDir, requestName)
}

// Start installs/configures system Tailscale and brings the node up.
func (m *Manager) Start(cfg Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	hostname := strings.TrimSpace(cfg.Hostname)
	if hostname == "" {
		hostname = "boomerang"
	}
	if strings.TrimSpace(cfg.DataDir) == "" {
		return fmt.Errorf("data dir required")
	}
	m.dataDir = cfg.DataDir

	// Drop legacy embedded-tsnet state so we don't keep a second identity around.
	_ = os.RemoveAll(StateDir(cfg.DataDir))

	req := map[string]string{
		"hostname": hostname,
		"authKey":  strings.TrimSpace(cfg.AuthKey),
	}
	body, _ := json.Marshal(req)
	path := m.requestPath(cfg.DataDir)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		m.lastErr = err.Error()
		return err
	}

	if err := m.runHelper("up"); err != nil {
		_ = os.Remove(path)
		m.lastErr = err.Error()
		return err
	}
	m.applySocksFromStatus()
	m.lastErr = ""
	return nil
}

// Stop disconnects from the Tailnet but keeps node state.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.runHelper("down"); err != nil {
		m.lastErr = err.Error()
		return err
	}
	tsdial.SetSOCKS("")
	m.lastErr = ""
	return nil
}

// Forget logs out, stops Tailscale, and clears state.
func (m *Manager) Forget(dataDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dataDir = dataDir
	_ = os.RemoveAll(StateDir(dataDir))
	_ = os.Remove(m.requestPath(dataDir))
	if err := m.runHelper("forget"); err != nil {
		m.lastErr = err.Error()
		return err
	}
	tsdial.SetSOCKS("")
	m.lastErr = ""
	return nil
}

// Status returns current Tailscale status and refreshes SOCKS routing.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := Status{}
	raw, err := m.runHelperOutput("status")
	if err != nil {
		out.LastError = err.Error()
		if m.lastErr != "" {
			out.LastError = m.lastErr
		}
		return out
	}
	var st helperStatus
	if err := json.Unmarshal(raw, &st); err != nil {
		out.LastError = err.Error()
		return out
	}
	out.Running = st.Running || st.Connected
	out.Connected = st.Connected
	out.Hostname = st.Hostname
	out.DNSName = st.DNSName
	out.IPs = st.IPs
	out.BackendState = st.BackendState
	out.Mode = st.Mode
	out.SocksAddr = st.SocksAddr
	out.TunAvailable = st.TunAvailable
	out.LastError = m.lastErr
	if st.SocksAddr != "" && st.Connected {
		tsdial.SetSOCKS(st.SocksAddr)
	} else if st.Mode == "tun" && st.Connected {
		tsdial.SetSOCKS("")
	}
	port := "8080"
	if out.DNSName != "" {
		out.URLs = append(out.URLs, "http://"+out.DNSName+":"+port)
	}
	for _, ip := range out.IPs {
		if strings.Contains(ip, ":") {
			continue
		}
		out.URLs = append(out.URLs, "http://"+ip+":"+port)
	}
	return out
}

func (m *Manager) Running() bool {
	st := m.Status()
	return st.Running || st.Connected
}

type helperStatus struct {
	Mode         string   `json:"mode"`
	SocksAddr    string   `json:"socksAddr"`
	Running      bool     `json:"running"`
	Connected    bool     `json:"connected"`
	BackendState string   `json:"backendState"`
	DNSName      string   `json:"dnsName"`
	IPs          []string `json:"ips"`
	Hostname     string   `json:"hostname"`
	TunAvailable bool     `json:"tunAvailable"`
}

func (m *Manager) applySocksFromStatus() {
	raw, err := m.runHelperOutput("status")
	if err != nil {
		return
	}
	var st helperStatus
	if json.Unmarshal(raw, &st) != nil {
		return
	}
	if st.SocksAddr != "" && st.Connected {
		tsdial.SetSOCKS(st.SocksAddr)
	} else {
		tsdial.SetSOCKS("")
	}
}

func (m *Manager) runHelper(arg string) error {
	_, err := m.runHelperOutput(arg)
	return err
}

func (m *Manager) runHelperOutput(arg string) ([]byte, error) {
	if _, err := os.Stat(helperPath); err != nil {
		return nil, fmt.Errorf("Tailscale helper missing (%s) — upgrade the appliance", helperPath)
	}
	ctx := exec.Command("sudo", "-n", helperPath, arg)
	ctx.Env = append(os.Environ(), "BOOMERANG_DATA_DIR="+m.dataDirOrDefault())
	out, err := ctx.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return out, nil
}

func (m *Manager) dataDirOrDefault() string {
	if m.dataDir != "" {
		return m.dataDir
	}
	return "/var/lib/boomerang"
}

// RefreshSOCKS loads SOCKS settings from the helper (call after reboot auto-start).
func (m *Manager) RefreshSOCKS(dataDir string) {
	m.mu.Lock()
	m.dataDir = dataDir
	m.mu.Unlock()
	// Give tailscaled a moment after boot.
	time.Sleep(100 * time.Millisecond)
	m.applySocksFromStatus()
}
