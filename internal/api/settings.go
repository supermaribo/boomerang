package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/boomerang-backup/boomerang/internal/jobs"
	"github.com/boomerang-backup/boomerang/internal/mysqlbackup"
	"github.com/boomerang-backup/boomerang/internal/notify"
	"github.com/boomerang-backup/boomerang/internal/store"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) SetScheduler(sched *jobs.Scheduler) {
	s.sched = sched
}

func (s *Server) routesExtra(r chi.Router) {
	r.Get("/backups/recent", s.handleRecentBackups)
	r.Get("/jobs", s.handleListJobs)
	r.Get("/settings", s.handleGetSettings)
	r.Put("/settings", s.handlePutSettings)
	r.Post("/settings/test-email", s.handleTestEmail)
	r.Post("/settings/password", s.handleChangePassword)
	r.Get("/appliance", s.handleGetAppliance)
	r.Post("/databases/{id}/versions/{vid}/restore", s.handleRestoreDatabase)
	r.Get("/databases/{id}/versions/{vid}", s.handleGetDBVersion)
	r.Get("/databases/{id}/versions/{vid}/tables", s.handleDBVersionTables)
	r.Delete("/databases/{id}/versions/{vid}", s.handleDeleteDBVersion)
}

func (s *Server) handleRecentBackups(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	versions, err := s.store.ListRecentVersions(limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	files, _ := s.store.ListFileServers()
	dbs, _ := s.store.ListDatabases()
	fname := map[string]string{}
	for _, f := range files {
		fname[f.ID] = f.Name
	}
	dname := map[string]string{}
	for _, d := range dbs {
		dname[d.ID] = d.Name
	}
	out := make([]map[string]any, 0, len(versions))
	for _, v := range versions {
		name := v.TargetID
		url := ""
		if v.TargetType == "file" {
			name = fname[v.TargetID]
			if name == "" {
				name = v.TargetID
			}
			url = "/app/file-servers/" + v.TargetID + "/backups"
		} else if v.TargetType == "db" {
			name = dname[v.TargetID]
			if name == "" {
				name = v.TargetID
			}
			url = "/app/databases"
		}
		out = append(out, map[string]any{
			"id": v.ID, "targetType": v.TargetType, "targetId": v.TargetID,
			"status": v.Status, "bytes": v.Bytes, "createdAt": v.CreatedAt,
			"targetName": name, "exploreUrl": url,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := s.store.ListRecentJobs(limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type settingsDTO struct {
	MailMode            string `json:"mailMode"`
	NotifyTo            string `json:"notifyTo"`
	NotifyFrom          string `json:"notifyFrom"`
	SMTPHost            string `json:"smtpHost"`
	SMTPPort            int    `json:"smtpPort"`
	SMTPUser            string `json:"smtpUser"`
	SMTPPassword        string `json:"smtpPassword,omitempty"`
	SMTPFrom            string `json:"smtpFrom"`
	SMTPTo              string `json:"smtpTo"`
	HasSMTPPass         bool   `json:"hasSmtpPassword"`
	AlertBackupSuccess  bool   `json:"alertBackupSuccess"`
	AlertBackupFailure  bool   `json:"alertBackupFailure"`
	AlertRestoreSuccess bool   `json:"alertRestoreSuccess"`
	AlertRestoreFailure bool   `json:"alertRestoreFailure"`
}

func (s *Server) LoadMail() (notify.MailConfig, error) {
	return s.loadMail()
}

func (s *Server) LoadSMTP() (notify.SMTPConfig, error) {
	cfg, err := s.loadMail()
	if err != nil {
		return notify.SMTPConfig{}, err
	}
	return cfg.SMTP, nil
}

func (s *Server) loadMail() (notify.MailConfig, error) {
	get := func(k, def string) string {
		v, ok, _ := s.store.GetMeta(k)
		if !ok || v == "" {
			return def
		}
		return v
	}
	boolMeta := func(k string, def bool) bool {
		v := get(k, "")
		if v == "" {
			return def
		}
		return v == "1" || strings.ToLower(v) == "true"
	}
	port := 587
	if p := get("smtp_port", "587"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}
	smtpCfg := notify.SMTPConfig{
		Host:     get("smtp_host", ""),
		Port:     port,
		Username: get("smtp_user", ""),
		From:     get("smtp_from", ""),
		To:       get("smtp_to", ""),
	}
	if hexEnc, ok, _ := s.store.GetMeta("smtp_password_sealed"); ok && hexEnc != "" {
		raw, err := hex.DecodeString(hexEnc)
		if err == nil {
			plain, err := s.box.Open(raw)
			if err == nil {
				smtpCfg.Password = string(plain)
			}
		}
	}
	mode := notify.MailMode(get("mail_mode", "local"))
	if mode != notify.MailSMTP {
		mode = notify.MailLocal
	}
	notifyTo := get("notify_to", "")
	if notifyTo == "" {
		notifyTo = smtpCfg.To // migrate old installs
	}
	return notify.MailConfig{
		Mode: mode,
		To:   notifyTo,
		From: get("notify_from", ""),
		SMTP: smtpCfg,
		Alerts: notify.AlertPrefs{
			BackupSuccess:  boolMeta("alert_backup_success", false),
			BackupFailure:  boolMeta("alert_backup_failure", true),
			RestoreSuccess: boolMeta("alert_restore_success", false),
			RestoreFailure: boolMeta("alert_restore_failure", true),
		},
	}, nil
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Current string `json:"currentPassword"`
		New     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.New) < 8 {
		writeErr(w, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}
	hash, ok, err := s.store.GetMeta("admin_password_hash")
	if err != nil || !ok {
		writeErr(w, http.StatusInternalServerError, "no admin")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Current)) != nil {
		writeErr(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.New), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "hash failed")
		return
	}
	if err := s.store.SetMeta("admin_password_hash", string(newHash)); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.BumpSessionEpoch()
	_ = s.store.Audit("password_change", "")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func settingsToDTO(cfg notify.MailConfig) settingsDTO {
	return settingsDTO{
		MailMode:            string(cfg.Mode),
		NotifyTo:            cfg.To,
		NotifyFrom:          cfg.From,
		SMTPHost:            cfg.SMTP.Host,
		SMTPPort:            cfg.SMTP.Port,
		SMTPUser:            cfg.SMTP.Username,
		SMTPFrom:            cfg.SMTP.From,
		SMTPTo:              cfg.SMTP.To,
		HasSMTPPass:         cfg.SMTP.Password != "",
		AlertBackupSuccess:  cfg.Alerts.BackupSuccess,
		AlertBackupFailure:  cfg.Alerts.BackupFailure,
		AlertRestoreSuccess: cfg.Alerts.RestoreSuccess,
		AlertRestoreFailure: cfg.Alerts.RestoreFailure,
	}
}

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	cfg, _ := s.loadMail()
	writeJSON(w, http.StatusOK, settingsToDTO(cfg))
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.SMTPPort == 0 {
		req.SMTPPort = 587
	}
	mode := strings.ToLower(strings.TrimSpace(req.MailMode))
	if mode != "smtp" {
		mode = "local"
	}
	_ = s.store.SetMeta("mail_mode", mode)
	_ = s.store.SetMeta("notify_to", strings.TrimSpace(req.NotifyTo))
	_ = s.store.SetMeta("notify_from", strings.TrimSpace(req.NotifyFrom))
	_ = s.store.SetMeta("alert_backup_success", boolStr(req.AlertBackupSuccess))
	_ = s.store.SetMeta("alert_backup_failure", boolStr(req.AlertBackupFailure))
	_ = s.store.SetMeta("alert_restore_success", boolStr(req.AlertRestoreSuccess))
	_ = s.store.SetMeta("alert_restore_failure", boolStr(req.AlertRestoreFailure))
	_ = s.store.SetMeta("smtp_host", req.SMTPHost)
	_ = s.store.SetMeta("smtp_port", strconv.Itoa(req.SMTPPort))
	_ = s.store.SetMeta("smtp_user", req.SMTPUser)
	_ = s.store.SetMeta("smtp_from", req.SMTPFrom)
	_ = s.store.SetMeta("smtp_to", req.SMTPTo)
	if req.SMTPPassword != "" {
		sealed, err := s.box.Seal([]byte(req.SMTPPassword))
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = s.store.SetMeta("smtp_password_sealed", hex.EncodeToString(sealed))
	}
	if s.sched != nil {
		s.sched.Reload()
	}
	_ = s.store.Audit("settings_update", "smtp")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleTestEmail(w http.ResponseWriter, _ *http.Request) {
	cfg, err := s.loadMail()
	if err != nil || !cfg.Ready() {
		writeErr(w, http.StatusBadRequest, "set a notify email address first")
		return
	}
	if cfg.Mode == notify.MailSMTP && !cfg.SMTP.Enabled() {
		writeErr(w, http.StatusBadRequest, "custom SMTP requires host, from, and to")
		return
	}
	mode := "local mail"
	if cfg.Mode == notify.MailSMTP {
		mode = "SMTP"
	}
	if err := cfg.Send("[Boomerang] Test email", fmt.Sprintf("Boomerang email alerts are working (%s).", mode)); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func boolStr(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func (s *Server) handleGetDBVersion(w http.ResponseWriter, r *http.Request) {
	dbID := chi.URLParam(r, "id")
	vid := chi.URLParam(r, "vid")
	v, err := s.store.GetVersion(vid)
	if err != nil || v == nil || v.TargetType != "db" || v.TargetID != dbID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

type dbRestoreReq struct {
	ConfirmName string   `json:"confirmName"`
	Tables      []string `json:"tables"`
}

func (s *Server) handleDBVersionTables(w http.ResponseWriter, r *http.Request) {
	dbID := chi.URLParam(r, "id")
	vid := chi.URLParam(r, "vid")
	v, err := s.store.GetVersion(vid)
	if err != nil || v == nil || v.TargetType != "db" || v.TargetID != dbID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	tables, err := mysqlbackup.ReadManifestTables(v.PathOnDisk)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tables": tables})
}

func (s *Server) handleRestoreDatabase(w http.ResponseWriter, r *http.Request) {
	dbID := chi.URLParam(r, "id")
	vid := chi.URLParam(r, "vid")
	db, err := s.store.GetDatabase(dbID)
	if err != nil || db == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	var req dbRestoreReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.ConfirmName != db.Name {
		writeErr(w, http.StatusBadRequest, "type the database name to confirm restore")
		return
	}
	jobID, err := s.runner.StartDBRestore(dbID, vid, req.Tables)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.Audit("db_restore", dbID+":"+vid)
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": jobID})
}

func (s *Server) handleDeleteDBVersion(w http.ResponseWriter, r *http.Request) {
	dbID := chi.URLParam(r, "id")
	vid := chi.URLParam(r, "vid")
	db, err := s.store.GetDatabase(dbID)
	if err != nil || db == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	var req deleteVersionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.ConfirmName != db.Name {
		writeErr(w, http.StatusBadRequest, "type the database name to confirm delete")
		return
	}
	if err := s.store.DeleteVersion("db", dbID, vid); err != nil {
		if errors.Is(err, store.ErrVersionNotFound) {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.Audit("db_version_delete", dbID+":"+vid)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
