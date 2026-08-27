package api

import (
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/boomerang-backup/boomerang/internal/tailnet"
)

type tailscaleDTO struct {
	Enabled      bool     `json:"enabled"`
	Hostname     string   `json:"hostname"`
	HasAuthKey   bool     `json:"hasAuthKey"`
	HasState     bool     `json:"hasState"`
	Running      bool     `json:"running"`
	Connected    bool     `json:"connected"`
	DNSName      string   `json:"dnsName,omitempty"`
	IPs          []string `json:"ips,omitempty"`
	URLs         []string `json:"urls,omitempty"`
	BackendState string   `json:"backendState,omitempty"`
	LastError    string   `json:"lastError,omitempty"`
	Mode         string   `json:"mode,omitempty"`
	SocksAddr    string   `json:"socksAddr,omitempty"`
	TunAvailable bool     `json:"tunAvailable"`
}

type tailscaleWrite struct {
	Enabled  *bool  `json:"enabled"`
	Hostname string `json:"hostname"`
	AuthKey  string `json:"authKey"`
}

func (s *Server) handleGetTailscale(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.tailscaleDTO())
}

func (s *Server) handlePutTailscale(w http.ResponseWriter, r *http.Request) {
	var req tailscaleWrite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.saveTailscaleConfig(req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.Audit("tailscale_settings", "update")
	writeJSON(w, http.StatusOK, s.tailscaleDTO())
}

func (s *Server) handleTailscaleConnect(w http.ResponseWriter, r *http.Request) {
	if s.tailnet == nil {
		writeErr(w, http.StatusServiceUnavailable, "tailscale not available")
		return
	}
	var req tailscaleWrite
	_ = json.NewDecoder(r.Body).Decode(&req)
	enabled := true
	req.Enabled = &enabled
	if err := s.saveTailscaleConfig(req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg, err := s.loadTailscaleConfig()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.tailnet.Start(cfg); err != nil {
		_ = s.store.SetMeta("tailscale_enabled", "0")
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.SetMeta("tailscale_enabled", "1")
	_ = s.store.Audit("tailscale_connect", cfg.Hostname)
	writeJSON(w, http.StatusOK, s.tailscaleDTO())
}

func (s *Server) handleTailscaleDisconnect(w http.ResponseWriter, _ *http.Request) {
	if s.tailnet == nil {
		writeErr(w, http.StatusServiceUnavailable, "tailscale not available")
		return
	}
	_ = s.tailnet.Stop()
	_ = s.store.SetMeta("tailscale_enabled", "0")
	_ = s.store.Audit("tailscale_disconnect", "")
	writeJSON(w, http.StatusOK, s.tailscaleDTO())
}

func (s *Server) handleTailscaleForget(w http.ResponseWriter, _ *http.Request) {
	if s.tailnet == nil {
		writeErr(w, http.StatusServiceUnavailable, "tailscale not available")
		return
	}
	_ = s.tailnet.Forget(s.cfg.DataDir)
	_ = s.store.SetMeta("tailscale_enabled", "0")
	_ = s.store.SetMeta("tailscale_authkey_sealed", "")
	_ = s.store.Audit("tailscale_forget", "")
	writeJSON(w, http.StatusOK, s.tailscaleDTO())
}

func (s *Server) tailscaleDTO() tailscaleDTO {
	hostname := "boomerang"
	if v, ok, _ := s.store.GetMeta("tailscale_hostname"); ok && strings.TrimSpace(v) != "" {
		hostname = strings.TrimSpace(v)
	}
	enabled := false
	if v, ok, _ := s.store.GetMeta("tailscale_enabled"); ok {
		enabled = v == "1" || strings.EqualFold(v, "true")
	}
	hasKey := false
	if v, ok, _ := s.store.GetMeta("tailscale_authkey_sealed"); ok && v != "" {
		hasKey = true
	}
	hasState := false
	if s.tailnet != nil {
		st := s.tailnet.Status()
		hasState = st.Connected || st.Running
	}
	if !hasState {
		if entries, err := os.ReadDir(tailnet.StateDir(s.cfg.DataDir)); err == nil && len(entries) > 0 {
			hasState = true
		}
	}
	out := tailscaleDTO{
		Enabled:    enabled,
		Hostname:   hostname,
		HasAuthKey: hasKey,
		HasState:   hasState,
	}
	if s.tailnet != nil {
		st := s.tailnet.Status()
		out.Running = st.Running
		out.Connected = st.Connected
		out.DNSName = st.DNSName
		out.IPs = st.IPs
		out.URLs = st.URLs
		out.BackendState = st.BackendState
		out.LastError = st.LastError
		out.Mode = st.Mode
		out.SocksAddr = st.SocksAddr
		out.TunAvailable = st.TunAvailable
		if st.Hostname != "" {
			out.Hostname = st.Hostname
		}
	}
	return out
}

func (s *Server) saveTailscaleConfig(req tailscaleWrite) error {
	hostname := strings.TrimSpace(req.Hostname)
	if hostname == "" {
		if v, ok, _ := s.store.GetMeta("tailscale_hostname"); ok && strings.TrimSpace(v) != "" {
			hostname = strings.TrimSpace(v)
		} else {
			hostname = "boomerang"
		}
	}
	if err := s.store.SetMeta("tailscale_hostname", hostname); err != nil {
		return err
	}
	if req.Enabled != nil {
		val := "0"
		if *req.Enabled {
			val = "1"
		}
		_ = s.store.SetMeta("tailscale_enabled", val)
	}
	key := strings.TrimSpace(req.AuthKey)
	if key != "" {
		sealed, err := s.box.Seal([]byte(key))
		if err != nil {
			return err
		}
		if err := s.store.SetMeta("tailscale_authkey_sealed", hex.EncodeToString(sealed)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) loadTailscaleConfig() (tailnet.Config, error) {
	hostname := "boomerang"
	if v, ok, _ := s.store.GetMeta("tailscale_hostname"); ok && strings.TrimSpace(v) != "" {
		hostname = strings.TrimSpace(v)
	}
	cfg := tailnet.Config{
		Hostname: hostname,
		DataDir:  s.cfg.DataDir,
	}
	if hexEnc, ok, _ := s.store.GetMeta("tailscale_authkey_sealed"); ok && hexEnc != "" {
		raw, err := hex.DecodeString(hexEnc)
		if err != nil {
			return cfg, err
		}
		plain, err := s.box.Open(raw)
		if err != nil {
			return cfg, err
		}
		cfg.AuthKey = string(plain)
	}
	return cfg, nil
}

// StartTailscaleIfEnabled reconnects after reboot when previously enabled.
func (s *Server) StartTailscaleIfEnabled() {
	if s.tailnet == nil {
		return
	}
	s.tailnet.RefreshSOCKS(s.cfg.DataDir)
	v, ok, _ := s.store.GetMeta("tailscale_enabled")
	if !ok || !(v == "1" || strings.EqualFold(v, "true")) {
		return
	}
	st := s.tailnet.Status()
	if st.Connected {
		return
	}
	cfg, err := s.loadTailscaleConfig()
	if err != nil {
		log.Printf("tailscale auto-start config: %v", err)
		return
	}
	if err := s.tailnet.Start(cfg); err != nil {
		log.Printf("tailscale auto-start: %v", err)
	}
}
