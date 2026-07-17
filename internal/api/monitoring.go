package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/monitoring"
	"github.com/boomerang-backup/boomerang/internal/remote"
	"github.com/boomerang-backup/boomerang/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) routesMonitoring(r chi.Router) {
	r.Get("/monitoring/servers", s.handleListMonitoredServers)
	r.Post("/monitoring/servers", s.handleCreateMonitoredServer)
	r.Get("/monitoring/servers/{id}", s.handleGetMonitoredServer)
	r.Put("/monitoring/servers/{id}", s.handleUpdateMonitoredServer)
	r.Delete("/monitoring/servers/{id}", s.handleDeleteMonitoredServer)
	r.Post("/monitoring/servers/{id}/test", s.handleTestMonitoredServer)
	r.Post("/monitoring/servers/{id}/poll", s.handlePollMonitoredServer)
	r.Post("/monitoring/servers/{id}/rotate-key", s.handleRotateMonitoredKey)
	r.Get("/monitoring/servers/{id}/history", s.handleMonitoredHistory)
	r.Get("/monitoring/servers/{id}/logs/sources", s.handleMonitoredLogSources)
	r.Get("/monitoring/servers/{id}/logs", s.handleMonitoredLogs)
	r.Get("/monitoring/install-hint", s.handleMonitorInstallHint)
}

type monitoredServerDTO struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Host             string   `json:"host"`
	Port             int      `json:"port"`
	Username         string   `json:"username"`
	PublicKey        string   `json:"publicKey,omitempty"`
	SSHHostKey       string   `json:"sshHostKey,omitempty"`
	FileServerID     *string  `json:"fileServerId"`
	Enabled          bool     `json:"enabled"`
	PollIntervalSec  int      `json:"pollIntervalSec"`
	OfflineAfterSec  int      `json:"offlineAfterSec"`
	AlertCPUPercent  float64  `json:"alertCpuPercent"`
	AlertMemPercent  float64  `json:"alertMemPercent"`
	AlertDiskPercent float64  `json:"alertDiskPercent"`
	AlertLoadPerCPU  float64  `json:"alertLoadPerCpu"`
	AlertSustainSec  int      `json:"alertSustainSec"`
	AlertsEnabled    bool     `json:"alertsEnabled"`
	ClientVersion    string   `json:"clientVersion,omitempty"`
	LastSampleAt     string   `json:"lastSampleAt,omitempty"`
	LastPollAt       string   `json:"lastPollAt,omitempty"`
	LastPollError    string   `json:"lastPollError,omitempty"`
	Online           bool     `json:"online"`
	StatusDetail     string   `json:"statusDetail,omitempty"`
	UptimeSec        int64    `json:"uptimeSec,omitempty"`
	CPUPercent       float64  `json:"cpuPercent,omitempty"`
	MemPercent       float64  `json:"memPercent,omitempty"`
	Load1            float64  `json:"load1,omitempty"`
	NumCPU           int      `json:"numCpu,omitempty"`
	PrimaryDiskMount string   `json:"primaryDiskMount,omitempty"`
	PrimaryDiskPct   float64  `json:"primaryDiskPercent,omitempty"`
	NetIface         string   `json:"netIface,omitempty"`
	NetRxBps         *float64 `json:"netRxBps"`
	NetTxBps         *float64 `json:"netTxBps"`
	ActiveAlerts     []string `json:"activeAlerts"`
	InstallCommand   string   `json:"installCommand,omitempty"`
	CreatedAt        string   `json:"createdAt,omitempty"`
	UpdatedAt        string   `json:"updatedAt,omitempty"`
}

type monitoredWrite struct {
	Name             string  `json:"name"`
	Host             string  `json:"host"`
	Port             int     `json:"port"`
	Username         string  `json:"username"`
	FileServerID     *string `json:"fileServerId"`
	Enabled          *bool   `json:"enabled"`
	PollIntervalSec  int     `json:"pollIntervalSec"`
	OfflineAfterSec  int     `json:"offlineAfterSec"`
	AlertCPUPercent  float64 `json:"alertCpuPercent"`
	AlertMemPercent  float64 `json:"alertMemPercent"`
	AlertDiskPercent float64 `json:"alertDiskPercent"`
	AlertLoadPerCPU  float64 `json:"alertLoadPerCpu"`
	AlertSustainSec  int     `json:"alertSustainSec"`
	AlertsEnabled    *bool   `json:"alertsEnabled"`
	PrivateKey       string  `json:"privateKey,omitempty"`
	PublicKey        string  `json:"publicKey,omitempty"`
}

func (s *Server) toMonitoredDTO(m store.MonitoredServer, includeInstall bool) monitoredServerDTO {
	now := time.Now().UTC()
	online, detail := monitoring.StatusFor(m, now)
	dto := monitoredServerDTO{
		ID: m.ID, Name: m.Name, Host: m.Host, Port: m.Port, Username: m.Username,
		SSHHostKey: m.SSHHostKey, Enabled: m.Enabled,
		PollIntervalSec: m.PollIntervalSec, OfflineAfterSec: m.OfflineAfterSec,
		AlertCPUPercent: m.AlertCPUPercent, AlertMemPercent: m.AlertMemPercent,
		AlertDiskPercent: m.AlertDiskPercent, AlertLoadPerCPU: m.AlertLoadPerCPU,
		AlertSustainSec: m.AlertSustainSec, AlertsEnabled: m.AlertsEnabled,
		ClientVersion: m.ClientVersion, LastPollError: m.LastPollError,
		Online: online, StatusDetail: detail,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		ActiveAlerts: []string{},
	}
	if m.FileServerID.Valid {
		v := m.FileServerID.String
		dto.FileServerID = &v
	}
	if m.LastSampleAt.Valid {
		dto.LastSampleAt = m.LastSampleAt.String
	}
	if m.LastPollAt.Valid {
		dto.LastPollAt = m.LastPollAt.String
	}
	if plain, err := s.box.Open(m.EncSecret); err == nil {
		if sec, err := remote.UnmarshalSecret(plain); err == nil {
			dto.PublicKey = sec.PublicKey
			if dto.PublicKey == "" && sec.PrivateKey != "" {
				dto.PublicKey, _ = remote.PublicKeyFromPrivate(sec.PrivateKey, sec.Passphrase)
			}
		}
	}
	if includeInstall && dto.PublicKey != "" {
		dto.InstallCommand = monitorInstallCommand(dto.PublicKey)
	}
	if sample, fs, err := s.store.LatestMonitorSample(m.ID); err == nil && sample != nil {
		dto.UptimeSec = sample.UptimeSec
		dto.CPUPercent = sample.CPUPercent
		dto.Load1 = sample.Load1
		dto.NumCPU = sample.NumCPU
		dto.NetIface = sample.NetIface
		dto.NetRxBps = sample.NetRxBps
		dto.NetTxBps = sample.NetTxBps
		if sample.MemTotalBytes > 0 {
			dto.MemPercent = 100 * float64(sample.MemUsedBytes) / float64(sample.MemTotalBytes)
		}
		bestPct := -1.0
		for _, f := range fs {
			if f.Mount == "/" || bestPct < 0 {
				if f.TotalBytes > 0 {
					pct := 100 * float64(f.UsedBytes) / float64(f.TotalBytes)
					if f.Mount == "/" || pct > bestPct {
						bestPct = pct
						dto.PrimaryDiskMount = f.Mount
						dto.PrimaryDiskPct = pct
					}
				}
			}
		}
	}
	if alerts, err := s.store.ListActiveMonitorAlerts(m.ID); err == nil {
		for _, a := range alerts {
			if strings.HasSuffix(a.AlertKey, "_pending") {
				continue
			}
			dto.ActiveAlerts = append(dto.ActiveAlerts, a.AlertKey)
		}
	}
	return dto
}

func monitorInstallCommand(pubKey string) string {
	pubKey = strings.TrimSpace(pubKey)
	return "curl -fsSL https://raw.githubusercontent.com/supermaribo/boomerang/main/deploy/monitor/install.sh | sudo bash -s -- --public-key '" + pubKey + "'"
}

func (s *Server) handleMonitorInstallHint(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"scriptURL": "https://raw.githubusercontent.com/supermaribo/boomerang/main/deploy/monitor/install.sh",
		"user":      "boomerang-monitor",
		"note":      "Run the install command on a Linux VPS with sudo. Requires root once to create the monitoring user and systemd service.",
	})
}

func (s *Server) handleListMonitoredServers(w http.ResponseWriter, _ *http.Request) {
	list, err := s.store.ListMonitoredServers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]monitoredServerDTO, 0, len(list))
	for _, m := range list {
		out = append(out, s.toMonitoredDTO(m, false))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetMonitoredServer(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.GetMonitoredServer(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if m == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, s.toMonitoredDTO(*m, true))
}

func (s *Server) handleCreateMonitoredServer(w http.ResponseWriter, r *http.Request) {
	var req monitoredWrite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	m, err := s.buildMonitoredServer("", req, true)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.UpsertMonitoredServer(m); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.Audit("monitor_create", m.ID)
	writeJSON(w, http.StatusCreated, s.toMonitoredDTO(*m, true))
}

func (s *Server) handleUpdateMonitoredServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := s.store.GetMonitoredServer(id)
	if err != nil || existing == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	var req monitoredWrite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	m, err := s.buildMonitoredServer(id, req, false)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	// Preserve secrets/status if not regenerating key.
	if len(m.EncSecret) == 0 {
		m.EncSecret = existing.EncSecret
	}
	m.SSHHostKey = existing.SSHHostKey
	m.ClientVersion = existing.ClientVersion
	m.LastSampleAt = existing.LastSampleAt
	m.LastPollAt = existing.LastPollAt
	m.LastPollError = existing.LastPollError
	m.LastBootID = existing.LastBootID
	if err := s.store.UpsertMonitoredServer(m); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.Audit("monitor_update", id)
	updated, _ := s.store.GetMonitoredServer(id)
	writeJSON(w, http.StatusOK, s.toMonitoredDTO(*updated, true))
}

func (s *Server) handleDeleteMonitoredServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		ConfirmName string `json:"confirmName"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	m, err := s.store.GetMonitoredServer(id)
	if err != nil || m == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if req.ConfirmName != "" && req.ConfirmName != m.Name {
		writeErr(w, http.StatusBadRequest, "type the server name to confirm delete")
		return
	}
	if err := s.store.DeleteMonitoredServer(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.Audit("monitor_delete", id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleTestMonitoredServer(w http.ResponseWriter, r *http.Request) {
	if s.monitor == nil {
		writeErr(w, http.StatusServiceUnavailable, "monitoring unavailable")
		return
	}
	msg, err := s.monitor.TestConnection(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg})
}

func (s *Server) handlePollMonitoredServer(w http.ResponseWriter, r *http.Request) {
	if s.monitor == nil {
		writeErr(w, http.StatusServiceUnavailable, "monitoring unavailable")
		return
	}
	if err := s.monitor.PollOne(chi.URLParam(r, "id")); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	m, _ := s.store.GetMonitoredServer(chi.URLParam(r, "id"))
	writeJSON(w, http.StatusOK, s.toMonitoredDTO(*m, true))
}

func (s *Server) handleMonitoredLogs(w http.ResponseWriter, r *http.Request) {
	if s.monitor == nil {
		writeErr(w, http.StatusServiceUnavailable, "monitoring unavailable")
		return
	}
	id := chi.URLParam(r, "id")
	lines := 200
	if raw := r.URL.Query().Get("lines"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			lines = n
		}
	}
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	text, err := s.monitor.FetchLogs(id, lines, source)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"lines":  lines,
		"source": source,
		"text":   text,
	})
}

func (s *Server) handleMonitoredLogSources(w http.ResponseWriter, r *http.Request) {
	if s.monitor == nil {
		writeErr(w, http.StatusServiceUnavailable, "monitoring unavailable")
		return
	}
	sources, err := s.monitor.FetchLogSources(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": sources})
}

func (s *Server) handleRotateMonitoredKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := s.store.GetMonitoredServer(id)
	if err != nil || m == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	priv, pub, err := remote.GenerateEd25519Keypair()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	sec := remote.AuthSecret{PrivateKey: priv, PublicKey: pub}
	raw, err := remote.MarshalSecret(sec)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	sealed, err := s.box.Seal(raw)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	m.EncSecret = sealed
	m.SSHHostKey = "" // force re-pin on next connect after reinstall
	if err := s.store.UpsertMonitoredServer(m); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.Audit("monitor_rotate_key", id)
	updated, _ := s.store.GetMonitoredServer(id)
	writeJSON(w, http.StatusOK, s.toMonitoredDTO(*updated, true))
}

func (s *Server) handleMonitoredHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := s.store.GetMonitoredServer(id)
	if err != nil || m == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	rangeName := r.URL.Query().Get("range")
	now := time.Now().UTC()
	var since time.Time
	useHourly := false
	switch rangeName {
	case "7d":
		since = now.Add(-7 * 24 * time.Hour)
		useHourly = true
	case "30d":
		since = now.Add(-30 * 24 * time.Hour)
		useHourly = true
	default:
		rangeName = "24h"
		since = now.Add(-24 * time.Hour)
	}

	sample, fs, _ := s.store.LatestMonitorSample(id)
	out := map[string]any{
		"range":       rangeName,
		"server":      s.toMonitoredDTO(*m, false),
		"filesystems": fs,
	}
	if sample != nil {
		out["latest"] = sample
		if sample.NetIface != "" {
			out["netIface"] = sample.NetIface
		}
	}
	if useHourly {
		rows, err := s.store.ListMonitorHourly(id, since, now)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		points := make([]map[string]any, 0, len(rows))
		for _, h := range rows {
			points = append(points, map[string]any{
				"at": h.HourAt, "cpu": h.AvgCPUPercent, "cpuMax": h.MaxCPUPercent,
				"mem": h.AvgMemPercent, "memMax": h.MaxMemPercent,
				"load1": h.AvgLoad1, "disk": h.MaxDiskPercent, "samples": h.Samples,
				"netRxBps": h.AvgNetRxBps, "netTxBps": h.AvgNetTxBps,
				"netRxBpsMax": h.MaxNetRxBps, "netTxBpsMax": h.MaxNetTxBps,
			})
		}
		out["points"] = points
	} else {
		rows, err := s.store.ListMonitorSamples(id, since, now)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		points := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			mem := 0.0
			if row.MemTotalBytes > 0 {
				mem = 100 * float64(row.MemUsedBytes) / float64(row.MemTotalBytes)
			}
			pt := map[string]any{
				"at": row.SampledAt, "cpu": row.CPUPercent, "mem": mem,
				"load1": row.Load1, "uptimeSec": row.UptimeSec, "numCpu": row.NumCPU,
			}
			if row.NetIface != "" {
				pt["netIface"] = row.NetIface
			}
			if row.NetRxBps != nil {
				pt["netRxBps"] = *row.NetRxBps
			}
			if row.NetTxBps != nil {
				pt["netTxBps"] = *row.NetTxBps
			}
			points = append(points, pt)
		}
		out["points"] = points
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) buildMonitoredServer(id string, req monitoredWrite, isNew bool) (*store.MonitoredServer, error) {
	name := strings.TrimSpace(req.Name)
	host := strings.TrimSpace(req.Host)
	if name == "" || host == "" {
		return nil, fmt.Errorf("name and host are required")
	}
	if id == "" {
		id = uuid.NewString()
	}
	port := req.Port
	if port <= 0 {
		port = 22
	}
	user := strings.TrimSpace(req.Username)
	if user == "" {
		user = "boomerang-monitor"
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	alerts := true
	if req.AlertsEnabled != nil {
		alerts = *req.AlertsEnabled
	}
	m := &store.MonitoredServer{
		ID: id, Name: name, Host: host, Port: port, Username: user,
		Enabled: enabled, AlertsEnabled: alerts,
		PollIntervalSec: req.PollIntervalSec, OfflineAfterSec: req.OfflineAfterSec,
		AlertCPUPercent: req.AlertCPUPercent, AlertMemPercent: req.AlertMemPercent,
		AlertDiskPercent: req.AlertDiskPercent, AlertLoadPerCPU: req.AlertLoadPerCPU,
		AlertSustainSec: req.AlertSustainSec,
	}
	if m.PollIntervalSec <= 0 {
		m.PollIntervalSec = 60
	}
	if m.OfflineAfterSec <= 0 {
		m.OfflineAfterSec = 180
	}
	if m.AlertCPUPercent <= 0 {
		m.AlertCPUPercent = 90
	}
	if m.AlertMemPercent <= 0 {
		m.AlertMemPercent = 90
	}
	if m.AlertDiskPercent <= 0 {
		m.AlertDiskPercent = 90
	}
	if m.AlertLoadPerCPU <= 0 {
		m.AlertLoadPerCPU = 2
	}
	if m.AlertSustainSec <= 0 {
		m.AlertSustainSec = 300
	}
	if req.FileServerID != nil && strings.TrimSpace(*req.FileServerID) != "" {
		m.FileServerID = sql.NullString{String: strings.TrimSpace(*req.FileServerID), Valid: true}
	}

	priv := strings.TrimSpace(req.PrivateKey)
	pub := strings.TrimSpace(req.PublicKey)
	if isNew || priv != "" {
		if priv == "" {
			var err error
			priv, pub, err = remote.GenerateEd25519Keypair()
			if err != nil {
				return nil, err
			}
		} else if pub == "" {
			var err error
			pub, err = remote.PublicKeyFromPrivate(priv, "")
			if err != nil {
				return nil, fmt.Errorf("invalid private key: %w", err)
			}
		}
		sec := remote.AuthSecret{PrivateKey: priv, PublicKey: pub}
		raw, err := remote.MarshalSecret(sec)
		if err != nil {
			return nil, err
		}
		sealed, err := s.box.Seal(raw)
		if err != nil {
			return nil, err
		}
		m.EncSecret = sealed
	}
	return m, nil
}
