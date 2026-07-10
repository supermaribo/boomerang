package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/boomerang-backup/boomerang/internal/mysqlbackup"
	"github.com/boomerang-backup/boomerang/internal/remote"
	"github.com/boomerang-backup/boomerang/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) routesTargets(r chi.Router) {
	r.Get("/file-servers", s.handleListFileServers)
	r.Post("/file-servers", s.handleCreateFileServer)
	r.Get("/file-servers/{id}", s.handleGetFileServer)
	r.Put("/file-servers/{id}", s.handleUpdateFileServer)
	r.Delete("/file-servers/{id}", s.handleDeleteFileServer)
	r.Post("/file-servers/test", s.handleTestFileServer)
	r.Post("/file-servers/browse", s.handleBrowseFileServer)
	r.Post("/file-servers/{id}/test", s.handleTestFileServerByID)
	r.Post("/file-servers/{id}/browse", s.handleBrowseFileServerByID)
	r.Post("/file-servers/{id}/backup", s.handleBackupFileServer)
	r.Get("/file-servers/{id}/versions", s.handleListFileVersions)
	r.Get("/file-servers/{id}/versions/{vid}", s.handleGetFileVersion)
	r.Get("/file-servers/{id}/versions/{vid}/tree", s.handleFileVersionTree)
	r.Post("/file-servers/{id}/versions/{vid}/restore", s.handleRestoreFileVersion)
	r.Post("/file-servers/{id}/versions/{vid}/download", s.handleDownloadFileVersion)
	r.Post("/keys/generate", s.handleGenerateKey)

	r.Get("/databases", s.handleListDatabases)
	r.Post("/databases", s.handleCreateDatabase)
	r.Get("/databases/{id}", s.handleGetDatabase)
	r.Put("/databases/{id}", s.handleUpdateDatabase)
	r.Delete("/databases/{id}", s.handleDeleteDatabase)
	r.Post("/databases/{id}/backup", s.handleBackupDatabase)
	r.Post("/databases/browse-tables", s.handleBrowseDatabaseTables)
	r.Post("/databases/{id}/browse-tables", s.handleBrowseDatabaseTablesByID)
	r.Get("/databases/{id}/versions", s.handleListDBVersions)

	r.Get("/jobs/{id}", s.handleGetJob)
	r.Get("/jobs/{id}/logs", s.handleGetJobLogs)
}

type fileServerDTO struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Protocol      string   `json:"protocol"`
	Host          string   `json:"host"`
	Port          int      `json:"port"`
	Username      string   `json:"username"`
	RemoteRoot    string   `json:"remoteRoot"`
	IncludePaths  []string `json:"includePaths"`
	ExcludePaths  []string `json:"excludePaths"`
	AuthMode      string   `json:"authMode"`
	ScheduleCron  string   `json:"scheduleCron"`
	ScheduleStart string   `json:"scheduleStart"`
	RetainCount   int      `json:"retainCount"`
	RetainDays    int      `json:"retainDays"`
	RetainHourly  int      `json:"retainHourly"`
	RetainDaily   int      `json:"retainDaily"`
	RetainWeekly  int      `json:"retainWeekly"`
	RetainYearly  int      `json:"retainYearly"`
	Enabled       bool     `json:"enabled"`
	HasSecret     bool     `json:"hasSecret"`
	PublicKey     string   `json:"publicKey,omitempty"`
	CreatedAt     string   `json:"createdAt,omitempty"`
	UpdatedAt     string   `json:"updatedAt,omitempty"`
}

type fileServerWrite struct {
	Name          string `json:"name"`
	Protocol      string `json:"protocol"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	RemoteRoot    string   `json:"remoteRoot"`
	IncludePaths  []string `json:"includePaths"`
	ExcludePaths  []string `json:"excludePaths"`
	AuthMode      string   `json:"authMode"`
	Password      string   `json:"password,omitempty"`
	PrivateKey    string   `json:"privateKey,omitempty"`
	Passphrase    string   `json:"passphrase,omitempty"`
	PublicKey     string   `json:"publicKey,omitempty"`
	ScheduleCron  string   `json:"scheduleCron"`
	ScheduleStart string   `json:"scheduleStart"`
	RetainCount   int      `json:"retainCount"`
	RetainDays    int      `json:"retainDays"`
	RetainHourly  int      `json:"retainHourly"`
	RetainDaily   int      `json:"retainDaily"`
	RetainWeekly  int      `json:"retainWeekly"`
	RetainYearly  int      `json:"retainYearly"`
	Enabled       *bool    `json:"enabled"`
}

type databaseDTO struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	MysqlHost     string  `json:"mysqlHost"`
	MysqlPort     int     `json:"mysqlPort"`
	MysqlDB       string  `json:"mysqlDb"`
	MysqlUser     string  `json:"mysqlUser"`
	IncludeTables []string `json:"includeTables"`
	TunnelMode    string  `json:"tunnelMode"`
	FileServerID  *string `json:"fileServerId"`
	SSHHost       string  `json:"sshHost"`
	SSHPort       int     `json:"sshPort"`
	SSHUsername   string  `json:"sshUsername"`
	AuthMode      string  `json:"authMode"`
	ScheduleCron  string  `json:"scheduleCron"`
	ScheduleStart string  `json:"scheduleStart"`
	RetainCount   int     `json:"retainCount"`
	RetainDays    int     `json:"retainDays"`
	RetainHourly  int     `json:"retainHourly"`
	RetainDaily   int     `json:"retainDaily"`
	RetainWeekly  int     `json:"retainWeekly"`
	RetainYearly  int     `json:"retainYearly"`
	Enabled       bool    `json:"enabled"`
	HasMysqlPass  bool    `json:"hasMysqlPassword"`
	HasSSHSecret  bool    `json:"hasSshSecret"`
}

type databaseWrite struct {
	Name          string  `json:"name"`
	MysqlHost     string  `json:"mysqlHost"`
	MysqlPort     int     `json:"mysqlPort"`
	MysqlDB       string  `json:"mysqlDb"`
	MysqlUser     string   `json:"mysqlUser"`
	MysqlPassword string   `json:"mysqlPassword,omitempty"`
	IncludeTables []string `json:"includeTables"`
	TunnelMode    string   `json:"tunnelMode"`
	FileServerID  *string `json:"fileServerId"`
	SSHHost       string  `json:"sshHost"`
	SSHPort       int     `json:"sshPort"`
	SSHUsername   string  `json:"sshUsername"`
	AuthMode      string  `json:"authMode"`
	Password      string  `json:"password,omitempty"`
	PrivateKey    string  `json:"privateKey,omitempty"`
	Passphrase    string  `json:"passphrase,omitempty"`
	ScheduleCron  string  `json:"scheduleCron"`
	ScheduleStart string  `json:"scheduleStart"`
	RetainCount   int     `json:"retainCount"`
	RetainDays    int     `json:"retainDays"`
	RetainHourly  int     `json:"retainHourly"`
	RetainDaily   int     `json:"retainDaily"`
	RetainWeekly  int     `json:"retainWeekly"`
	RetainYearly  int     `json:"retainYearly"`
	Enabled       *bool   `json:"enabled"`
}

func toFileDTO(f store.FileServer) fileServerDTO {
	return fileServerDTO{
		ID: f.ID, Name: f.Name, Protocol: f.Protocol, Host: f.Host, Port: f.Port,
		Username: f.Username, RemoteRoot: f.RemoteRoot, IncludePaths: f.IncludePaths, ExcludePaths: f.ExcludePaths, AuthMode: f.AuthMode,
		ScheduleCron: f.ScheduleCron, ScheduleStart: f.ScheduleStart,
		RetainCount: f.RetainCount, RetainDays: f.RetainDays,
		RetainHourly: f.RetainHourly, RetainDaily: f.RetainDaily,
		RetainWeekly: f.RetainWeekly, RetainYearly: f.RetainYearly,
		Enabled: f.Enabled, HasSecret: len(f.EncSecret) > 0, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
	}
}

func (s *Server) fileDTOWithPublicKey(f store.FileServer) fileServerDTO {
	dto := toFileDTO(f)
	if len(f.EncSecret) == 0 {
		return dto
	}
	plain, err := s.box.Open(f.EncSecret)
	if err != nil {
		return dto
	}
	secret, err := remote.UnmarshalSecret(plain)
	if err != nil {
		return dto
	}
	if secret.PublicKey != "" {
		dto.PublicKey = secret.PublicKey
		return dto
	}
	if secret.PrivateKey != "" {
		if pub, err := remote.PublicKeyFromPrivate(secret.PrivateKey, secret.Passphrase); err == nil {
			dto.PublicKey = pub
		}
	}
	return dto
}

func toDBDTO(d store.Database) databaseDTO {
	var fsID *string
	if d.FileServerID.Valid {
		v := d.FileServerID.String
		fsID = &v
	}
	sshHost := ""
	if d.SSHHost.Valid {
		sshHost = d.SSHHost.String
	}
	sshUser := ""
	if d.SSHUsername.Valid {
		sshUser = d.SSHUsername.String
	}
	return databaseDTO{
		ID: d.ID, Name: d.Name, MysqlHost: d.MysqlHost, MysqlPort: d.MysqlPort, MysqlDB: d.MysqlDB,
		MysqlUser: d.MysqlUser, IncludeTables: d.IncludeTables, TunnelMode: d.TunnelMode, FileServerID: fsID, SSHHost: sshHost,
		SSHPort: d.SSHPort, SSHUsername: sshUser, AuthMode: d.AuthMode,
		ScheduleCron: d.ScheduleCron, ScheduleStart: d.ScheduleStart,
		RetainCount: d.RetainCount, RetainDays: d.RetainDays,
		RetainHourly: d.RetainHourly, RetainDaily: d.RetainDaily,
		RetainWeekly: d.RetainWeekly, RetainYearly: d.RetainYearly,
		Enabled: d.Enabled,
		HasMysqlPass: len(d.EncMysqlPassword) > 0, HasSSHSecret: len(d.EncSSHSecret) > 0,
	}
}

func (s *Server) handleListFileServers(w http.ResponseWriter, _ *http.Request) {
	list, err := s.store.ListFileServers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]fileServerDTO, 0, len(list))
	for _, f := range list {
		out = append(out, s.fileDTOWithPublicKey(f))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetFileServer(w http.ResponseWriter, r *http.Request) {
	f, err := s.store.GetFileServer(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if f == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, s.fileDTOWithPublicKey(*f))
}

func (s *Server) handleCreateFileServer(w http.ResponseWriter, r *http.Request) {
	var req fileServerWrite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	f, err := s.buildFileServer("", req, true)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.UpsertFileServer(f); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.Audit("file_server_create", f.ID)
	s.reloadSched()
	writeJSON(w, http.StatusCreated, s.fileDTOWithPublicKey(*f))
}

func (s *Server) handleUpdateFileServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := s.store.GetFileServer(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	var req fileServerWrite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	f, err := s.buildFileServer(id, req, false)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	f.CreatedAt = existing.CreatedAt
	if f.EncSecret == nil {
		f.EncSecret = existing.EncSecret
	}
	if err := s.store.UpsertFileServer(f); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.Audit("file_server_update", f.ID)
	s.reloadSched()
	writeJSON(w, http.StatusOK, s.fileDTOWithPublicKey(*f))
}

func (s *Server) handleDeleteFileServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.PurgeTarget("file", id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = os.RemoveAll(filepath.Join(s.cfg.DataDir, "backups", "files", id))
	if err := s.store.DeleteFileServer(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.Audit("file_server_delete", id)
	s.reloadSched()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) buildFileServer(id string, req fileServerWrite, requireSecret bool) (*store.FileServer, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	if req.Name == "" || req.Host == "" || req.Username == "" {
		return nil, errMsg("name, host, and username are required")
	}
	proto := strings.ToLower(strings.TrimSpace(req.Protocol))
	if proto == "" {
		proto = "sftp"
	}
	switch proto {
	case "sftp", "rsync", "ftp", "ftps":
	default:
		return nil, errMsg("protocol must be sftp, rsync, ftp, or ftps")
	}
	auth := strings.ToLower(strings.TrimSpace(req.AuthMode))
	if auth == "" {
		auth = "password"
	}
	if proto == "ftp" || proto == "ftps" {
		auth = "password"
	}
	if req.Port == 0 {
		if proto == "ftp" || proto == "ftps" {
			req.Port = 21
		} else {
			req.Port = 22
		}
	}
	if req.RemoteRoot == "" {
		req.RemoteRoot = "/"
	}
	if req.IncludePaths == nil {
		req.IncludePaths = []string{}
	}
	if req.ExcludePaths == nil {
		req.ExcludePaths = []string{}
	}
	if req.ScheduleCron == "" {
		req.ScheduleCron = "0 2 * * *"
	}
	if req.RetainHourly == 0 && req.RetainDaily == 0 && req.RetainWeekly == 0 && req.RetainYearly == 0 {
		if req.RetainCount == 0 && req.RetainDays == 0 {
			req.RetainHourly = 24
			req.RetainDaily = 7
			req.RetainWeekly = 4
			req.RetainYearly = 1
		}
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if id == "" {
		id = uuid.NewString()
	}
	f := &store.FileServer{
		ID: id, Name: req.Name, Protocol: proto, Host: req.Host, Port: req.Port,
		Username: req.Username, RemoteRoot: req.RemoteRoot, IncludePaths: req.IncludePaths, ExcludePaths: req.ExcludePaths, AuthMode: auth,
		ScheduleCron: req.ScheduleCron, ScheduleStart: req.ScheduleStart,
		RetainCount: req.RetainCount, RetainDays: req.RetainDays,
		RetainHourly: req.RetainHourly, RetainDaily: req.RetainDaily,
		RetainWeekly: req.RetainWeekly, RetainYearly: req.RetainYearly,
		Enabled: enabled,
	}
	secret := remote.AuthSecret{
		Password: req.Password, PrivateKey: req.PrivateKey, Passphrase: req.Passphrase, PublicKey: req.PublicKey,
	}
	if auth == "password" && req.Password == "" && requireSecret {
		return nil, errMsg("password is required")
	}
	if auth == "key" && req.PrivateKey == "" && requireSecret {
		return nil, errMsg("private key is required — generate a keypair first")
	}
	if auth == "key" && req.PrivateKey != "" && secret.PublicKey == "" {
		if pub, err := remote.PublicKeyFromPrivate(req.PrivateKey, req.Passphrase); err == nil {
			secret.PublicKey = pub
		}
	}
	if req.Password != "" || req.PrivateKey != "" {
		raw, err := remote.MarshalSecret(secret)
		if err != nil {
			return nil, err
		}
		sealed, err := s.box.Seal(raw)
		if err != nil {
			return nil, err
		}
		f.EncSecret = sealed
	}
	return f, nil
}

type testFileReq struct {
	fileServerWrite
	ID string `json:"id"`
}

func (s *Server) handleTestFileServer(w http.ResponseWriter, r *http.Request) {
	var req testFileReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	secret := remote.AuthSecret{Password: req.Password, PrivateKey: req.PrivateKey, Passphrase: req.Passphrase}
	msg, err := remote.TestFileTarget(remote.FileTarget{
		Protocol: strings.ToLower(req.Protocol), Host: req.Host, Port: req.Port,
		Username: req.Username, RemoteRoot: req.RemoteRoot, AuthMode: req.AuthMode, Secret: secret,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": msg})
}

type browseReq struct {
	fileServerWrite
	Path string `json:"path"`
}

func (s *Server) handleBrowseFileServer(w http.ResponseWriter, r *http.Request) {
	var req browseReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	secret := remote.AuthSecret{Password: req.Password, PrivateKey: req.PrivateKey, Passphrase: req.Passphrase}
	res, err := remote.Browse(remote.FileTarget{
		Protocol: strings.ToLower(req.Protocol), Host: req.Host, Port: req.Port,
		Username: req.Username, AuthMode: req.AuthMode, Secret: secret,
	}, req.Path)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleBrowseFileServerByID(w http.ResponseWriter, r *http.Request) {
	f, err := s.store.GetFileServer(chi.URLParam(r, "id"))
	if err != nil || f == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	plain, err := s.box.Open(f.EncSecret)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "decrypt secret failed")
		return
	}
	secret, err := remote.UnmarshalSecret(plain)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "secret corrupt")
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Path == "" {
		req.Path = r.URL.Query().Get("path")
	}
	res, err := remote.Browse(remote.FileTarget{
		Protocol: f.Protocol, Host: f.Host, Port: f.Port, Username: f.Username,
		AuthMode: f.AuthMode, Secret: secret,
	}, req.Path)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleTestFileServerByID(w http.ResponseWriter, r *http.Request) {
	f, err := s.store.GetFileServer(chi.URLParam(r, "id"))
	if err != nil || f == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	plain, err := s.box.Open(f.EncSecret)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "decrypt secret failed")
		return
	}
	secret, err := remote.UnmarshalSecret(plain)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "secret corrupt")
		return
	}
	msg, err := remote.TestFileTarget(remote.FileTarget{
		Protocol: f.Protocol, Host: f.Host, Port: f.Port, Username: f.Username,
		RemoteRoot: f.RemoteRoot, AuthMode: f.AuthMode, Secret: secret,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": msg})
}

func (s *Server) handleBackupFileServer(w http.ResponseWriter, r *http.Request) {
	if s.runner == nil {
		writeErr(w, http.StatusServiceUnavailable, "backup runner unavailable")
		return
	}
	id := chi.URLParam(r, "id")
	jobID, err := s.runner.StartFileBackup(id)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.Audit("file_backup", id)
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": jobID})
}

func (s *Server) handleListFileVersions(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListVersions("file", chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	j, err := s.store.GetJob(chi.URLParam(r, "id"))
	if err != nil || j == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, j)
}

func (s *Server) handleGetJobLogs(w http.ResponseWriter, r *http.Request) {
	lines, err := s.store.ListJobLogs(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lines": lines})
}

func (s *Server) handleGenerateKey(w http.ResponseWriter, _ *http.Request) {
	priv, pub, err := remote.GenerateEd25519Keypair()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"privateKey": priv,
		"publicKey":  pub,
	})
}

func (s *Server) handleListDatabases(w http.ResponseWriter, _ *http.Request) {
	list, err := s.store.ListDatabases()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]databaseDTO, 0, len(list))
	for _, d := range list {
		out = append(out, toDBDTO(d))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetDatabase(w http.ResponseWriter, r *http.Request) {
	d, err := s.store.GetDatabase(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if d == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, toDBDTO(*d))
}

func (s *Server) handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	var req databaseWrite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	d, err := s.buildDatabase("", req, true)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.UpsertDatabase(d); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.Audit("database_create", d.ID)
	s.reloadSched()
	writeJSON(w, http.StatusCreated, toDBDTO(*d))
}

func (s *Server) handleUpdateDatabase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := s.store.GetDatabase(id)
	if err != nil || existing == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	var req databaseWrite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	d, err := s.buildDatabase(id, req, false)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	d.CreatedAt = existing.CreatedAt
	if d.EncMysqlPassword == nil {
		d.EncMysqlPassword = existing.EncMysqlPassword
	}
	if d.EncSSHSecret == nil {
		d.EncSSHSecret = existing.EncSSHSecret
	}
	if err := s.store.UpsertDatabase(d); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.Audit("database_update", d.ID)
	s.reloadSched()
	writeJSON(w, http.StatusOK, toDBDTO(*d))
}

func (s *Server) handleDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.PurgeTarget("db", id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = os.RemoveAll(filepath.Join(s.cfg.DataDir, "backups", "db", id))
	if err := s.store.DeleteDatabase(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.Audit("database_delete", id)
	s.reloadSched()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) buildDatabase(id string, req databaseWrite, requireSecret bool) (*store.Database, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.MysqlDB = strings.TrimSpace(req.MysqlDB)
	req.MysqlUser = strings.TrimSpace(req.MysqlUser)
	if req.Name == "" || req.MysqlDB == "" || req.MysqlUser == "" {
		return nil, errMsg("name, mysqlDb, and mysqlUser are required")
	}
	if req.MysqlHost == "" {
		req.MysqlHost = "127.0.0.1"
	}
	if req.MysqlPort == 0 {
		req.MysqlPort = 3306
	}
	tunnel := strings.ToLower(strings.TrimSpace(req.TunnelMode))
	if tunnel == "" {
		tunnel = "none"
	}
	auth := strings.ToLower(strings.TrimSpace(req.AuthMode))
	if auth == "" {
		auth = "password"
	}
	if req.ScheduleCron == "" {
		req.ScheduleCron = "0 2 * * *"
	}
	if req.RetainHourly == 0 && req.RetainDaily == 0 && req.RetainWeekly == 0 && req.RetainYearly == 0 {
		if req.RetainCount == 0 && req.RetainDays == 0 {
			req.RetainHourly = 24
			req.RetainDaily = 7
			req.RetainWeekly = 4
			req.RetainYearly = 1
		}
	}
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.IncludeTables == nil {
		req.IncludeTables = []string{}
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if id == "" {
		id = uuid.NewString()
	}
	d := &store.Database{
		ID: id, Name: req.Name, MysqlHost: req.MysqlHost, MysqlPort: req.MysqlPort,
		MysqlDB: req.MysqlDB, MysqlUser: req.MysqlUser, IncludeTables: req.IncludeTables, TunnelMode: tunnel, AuthMode: auth,
		SSHPort: req.SSHPort, ScheduleCron: req.ScheduleCron, ScheduleStart: req.ScheduleStart,
		RetainCount: req.RetainCount, RetainDays: req.RetainDays,
		RetainHourly: req.RetainHourly, RetainDaily: req.RetainDaily,
		RetainWeekly: req.RetainWeekly, RetainYearly: req.RetainYearly,
		Enabled: enabled,
	}
	if req.FileServerID != nil && *req.FileServerID != "" {
		d.FileServerID = sql.NullString{String: *req.FileServerID, Valid: true}
	}
	if req.SSHHost != "" {
		d.SSHHost = sql.NullString{String: req.SSHHost, Valid: true}
	}
	if req.SSHUsername != "" {
		d.SSHUsername = sql.NullString{String: req.SSHUsername, Valid: true}
	}
	if requireSecret && req.MysqlPassword == "" {
		return nil, errMsg("mysqlPassword is required")
	}
	if req.MysqlPassword != "" {
		sealed, err := s.box.Seal([]byte(req.MysqlPassword))
		if err != nil {
			return nil, err
		}
		d.EncMysqlPassword = sealed
	}
	if tunnel != "none" && tunnel != "fileserver" {
		if requireSecret {
			if auth == "password" && req.Password == "" && tunnel == "inline" {
				return nil, errMsg("ssh password required for tunnel")
			}
			if auth == "key" && req.PrivateKey == "" && tunnel == "inline" {
				return nil, errMsg("ssh private key required for tunnel")
			}
		}
	}
	if req.Password != "" || req.PrivateKey != "" {
		raw, err := remote.MarshalSecret(remote.AuthSecret{
			Password: req.Password, PrivateKey: req.PrivateKey, Passphrase: req.Passphrase,
		})
		if err != nil {
			return nil, err
		}
		sealed, err := s.box.Seal(raw)
		if err != nil {
			return nil, err
		}
		d.EncSSHSecret = sealed
	}
	return d, nil
}

func (s *Server) handleBrowseDatabaseTables(w http.ResponseWriter, r *http.Request) {
	var req databaseWrite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	t, err := s.mysqlTargetFromWrite("", req, nil)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	tables, err := mysqlbackup.ListTables(t, nil)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tables": tables})
}

func (s *Server) handleBrowseDatabaseTablesByID(w http.ResponseWriter, r *http.Request) {
	existing, err := s.store.GetDatabase(chi.URLParam(r, "id"))
	if err != nil || existing == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	var req databaseWrite
	_ = json.NewDecoder(r.Body).Decode(&req)
	t, err := s.mysqlTargetFromWrite(existing.ID, req, existing)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	tables, err := mysqlbackup.ListTables(t, nil)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tables": tables})
}

func (s *Server) mysqlTargetFromWrite(id string, req databaseWrite, existing *store.Database) (mysqlbackup.Target, error) {
	d, err := s.buildDatabase(id, req, false)
	if err != nil {
		return mysqlbackup.Target{}, err
	}
	if existing != nil {
		if len(d.EncMysqlPassword) == 0 {
			d.EncMysqlPassword = existing.EncMysqlPassword
		}
		if len(d.EncSSHSecret) == 0 {
			d.EncSSHSecret = existing.EncSSHSecret
		}
	}
	if len(d.EncMysqlPassword) == 0 {
		return mysqlbackup.Target{}, errMsg("mysql password is required to list tables")
	}
	tunnel := strings.ToLower(strings.TrimSpace(req.TunnelMode))
	if tunnel == "" && existing != nil {
		tunnel = existing.TunnelMode
	}
	if tunnel == "inline" && len(d.EncSSHSecret) == 0 {
		return mysqlbackup.Target{}, errMsg("ssh credentials are required to list tables")
	}
	if tunnel == "fileserver" && !d.FileServerID.Valid {
		return mysqlbackup.Target{}, errMsg("select a file server for the ssh tunnel")
	}
	return s.runner.MySQLTarget(d)
}

type simpleError string

func (e simpleError) Error() string { return string(e) }

func errMsg(s string) error { return simpleError(s) }

func (s *Server) reloadSched() {
	if s.sched != nil {
		s.sched.Reload()
	}
}
