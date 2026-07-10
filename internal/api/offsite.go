package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/boomerang-backup/boomerang/internal/offsite"
)

type offsiteDTO struct {
	Enabled      bool   `json:"enabled"`
	AccountID    string `json:"accountId"`
	Bucket       string `json:"bucket"`
	Prefix       string `json:"prefix"`
	AccessKey    string `json:"accessKey,omitempty"`
	SecretKey    string `json:"secretKey,omitempty"`
	HasAccessKey bool   `json:"hasAccessKey"`
	HasSecretKey bool   `json:"hasSecretKey"`
	LastSync     string `json:"lastSync"`
	LastError    string `json:"lastError"`
	LastFiles    int    `json:"lastFiles"`
	LastBytes    int64  `json:"lastBytes"`
	Syncing      bool   `json:"syncing"`
}

func (s *Server) offsiteDTO() offsiteDTO {
	cfg, _ := offsite.LoadConfig(s.store, s.box)
	st := offsite.LoadStatus(s.store)
	syncing := st.Syncing
	if s.offsite != nil && s.offsite.IsSyncing() {
		syncing = true
	}
	return offsiteDTO{
		Enabled:      cfg.Enabled,
		AccountID:    cfg.AccountID,
		Bucket:       cfg.Bucket,
		Prefix:       cfg.Prefix,
		HasAccessKey: cfg.AccessKey != "",
		HasSecretKey: cfg.SecretKey != "",
		LastSync:     st.LastSync,
		LastError:    st.LastError,
		LastFiles:    st.LastFiles,
		LastBytes:    st.LastBytes,
		Syncing:      syncing,
	}
}

func (s *Server) handleGetOffsite(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.offsiteDTO())
}

func (s *Server) handlePutOffsite(w http.ResponseWriter, r *http.Request) {
	var req offsiteDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	cfg := offsite.Config{
		Enabled:   req.Enabled,
		AccountID: strings.TrimSpace(req.AccountID),
		Bucket:    strings.TrimSpace(req.Bucket),
		Prefix:    strings.TrimSpace(req.Prefix),
	}
	if err := offsite.SaveConfig(s.store, s.box, cfg, strings.TrimSpace(req.AccessKey), strings.TrimSpace(req.SecretKey)); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if req.Enabled && s.offsite != nil {
		s.offsite.Schedule()
	}
	_ = s.store.Audit("offsite_settings", cfg.Bucket)
	writeJSON(w, http.StatusOK, s.offsiteDTO())
}

func (s *Server) handleTestOffsite(w http.ResponseWriter, r *http.Request) {
	var req offsiteDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	cfg, _ := offsite.LoadConfig(s.store, s.box)
	if strings.TrimSpace(req.AccountID) != "" {
		cfg.AccountID = strings.TrimSpace(req.AccountID)
	}
	if strings.TrimSpace(req.Bucket) != "" {
		cfg.Bucket = strings.TrimSpace(req.Bucket)
	}
	if strings.TrimSpace(req.AccessKey) != "" {
		cfg.AccessKey = strings.TrimSpace(req.AccessKey)
	}
	if strings.TrimSpace(req.SecretKey) != "" {
		cfg.SecretKey = strings.TrimSpace(req.SecretKey)
	}
	if !cfg.Ready() {
		writeErr(w, http.StatusBadRequest, "enter account ID, bucket, and API keys")
		return
	}
	if err := offsite.TestConnection(cfg); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSyncOffsite(w http.ResponseWriter, _ *http.Request) {
	if s.offsite == nil {
		writeErr(w, http.StatusServiceUnavailable, "off-site sync not available")
		return
	}
	if s.offsite.IsSyncing() {
		writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "syncing": true})
		return
	}
	s.offsite.Schedule()
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "syncing": true})
}

func (s *Server) scheduleOffsiteMirror() {
	if s.offsite != nil {
		s.offsite.Schedule()
	}
}
