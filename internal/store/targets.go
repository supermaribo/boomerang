package store

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type FileServer struct {
	ID                 string
	Name               string
	Protocol           string
	Host               string
	Port               int
	Username           string
	RemoteRoot         string
	IncludePaths       []string
	ExcludePaths       []string
	AuthMode           string
	EncSecret          []byte
	ScheduleCron       string
	ScheduleStart      string
	RetainCount        int
	RetainDays         int
	RetainHourly       int
	RetainDaily        int
	RetainWeekly       int
	RetainMonthly      int
	RetainYearly       int
	IncrementalEnabled bool
	SkipIfUnchanged    bool
	SSHHostKey         string
	Enabled            bool
	CreatedAt          string
	UpdatedAt          string
}

type Database struct {
	ID               string
	Name             string
	MysqlHost        string
	MysqlPort        int
	MysqlDB          string
	MysqlUser        string
	EncMysqlPassword []byte
	IncludeTables    []string
	TunnelMode       string
	FileServerID     sql.NullString
	SSHHost          sql.NullString
	SSHPort          int
	SSHUsername      sql.NullString
	AuthMode         string
	EncSSHSecret     []byte
	ScheduleCron     string
	ScheduleStart    string
	RetainCount      int
	RetainDays       int
	RetainHourly     int
	RetainDaily      int
	RetainWeekly     int
	RetainMonthly    int
	RetainYearly     int
	SkipIfUnchanged  bool
	Enabled          bool
	CreatedAt        string
	UpdatedAt        string
}

func (s *Store) ListRecentVersions(limit int, targetType string) ([]Version, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows *sql.Rows
	var err error
	if targetType != "" {
		rows, err = s.DB.Query(`
		SELECT id, target_type, target_id, status, bytes, path_on_disk, created_at,
		       verified_at, verify_error
		FROM backup_versions
		WHERE target_type = ?
		ORDER BY created_at DESC
		LIMIT ?`, targetType, limit)
	} else {
		rows, err = s.DB.Query(`
		SELECT id, target_type, target_id, status, bytes, path_on_disk, created_at,
		       verified_at, verify_error
		FROM backup_versions
		ORDER BY created_at DESC
		LIMIT ?`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		var v Version
		var verified sql.NullString
		if err := rows.Scan(&v.ID, &v.TargetType, &v.TargetID, &v.Status, &v.Bytes, &v.PathOnDisk, &v.CreatedAt,
			&verified, &v.VerifyError); err != nil {
			return nil, err
		}
		if verified.Valid {
			v.VerifiedAt = verified.String
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) ListRecentJobs(limit int) ([]Job, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.DB.Query(`
		SELECT id, target_type, target_id, kind, status, error, started_at, finished_at, created_at
		FROM jobs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.TargetType, &j.TargetID, &j.Kind, &j.Status, &j.Error, &j.StartedAt, &j.FinishedAt, &j.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ListFailedJobsSince returns failed jobs created at or after since (UTC).
func (s *Store) ListFailedJobsSince(since time.Time) ([]Job, error) {
	rows, err := s.DB.Query(`
		SELECT id, target_type, target_id, kind, status, error, started_at, finished_at, created_at
		FROM jobs
		WHERE status='failed' AND created_at >= ?
		ORDER BY created_at DESC
		LIMIT 100`, since.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.TargetType, &j.TargetID, &j.Kind, &j.Status, &j.Error, &j.StartedAt, &j.FinishedAt, &j.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

const fileServerCols = `id, name, protocol, host, port, username, remote_root, include_paths, exclude_paths, auth_mode, enc_secret,
		       schedule_cron, schedule_start, retain_count, retain_days,
		       retain_hourly, retain_daily, retain_weekly, retain_monthly, retain_yearly,
		       incremental_enabled, skip_if_unchanged, ssh_host_key, enabled, created_at, updated_at`

func scanFileServer(scan func(dest ...any) error) (*FileServer, error) {
	var f FileServer
	var enabled, incr, skip int
	var includeJSON, excludeJSON string
	if err := scan(&f.ID, &f.Name, &f.Protocol, &f.Host, &f.Port, &f.Username, &f.RemoteRoot, &includeJSON, &excludeJSON,
		&f.AuthMode, &f.EncSecret, &f.ScheduleCron, &f.ScheduleStart, &f.RetainCount, &f.RetainDays,
		&f.RetainHourly, &f.RetainDaily, &f.RetainWeekly, &f.RetainMonthly, &f.RetainYearly,
		&incr, &skip, &f.SSHHostKey, &enabled, &f.CreatedAt, &f.UpdatedAt); err != nil {
		return nil, err
	}
	f.Enabled = enabled == 1
	f.IncrementalEnabled = incr == 1
	f.SkipIfUnchanged = skip == 1
	f.IncludePaths = decodePaths(includeJSON)
	f.ExcludePaths = decodePaths(excludeJSON)
	return &f, nil
}

func decodePaths(s string) []string {
	if strings.TrimSpace(s) == "" || s == "null" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return []string{}
	}
	if out == nil {
		return []string{}
	}
	return out
}

func encodePaths(paths []string) string {
	if len(paths) == 0 {
		return "[]"
	}
	b, err := json.Marshal(paths)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func (s *Store) ListFileServers() ([]FileServer, error) {
	rows, err := s.DB.Query(`SELECT ` + fileServerCols + ` FROM file_servers ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FileServer
	for rows.Next() {
		f, err := scanFileServer(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

func (s *Store) GetFileServer(id string) (*FileServer, error) {
	f, err := scanFileServer(s.DB.QueryRow(`SELECT `+fileServerCols+` FROM file_servers WHERE id = ?`, id).Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Store) UpsertFileServer(f *FileServer) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if f.CreatedAt == "" {
		f.CreatedAt = now
	}
	f.UpdatedAt = now
	en := 0
	if f.Enabled {
		en = 1
	}
	incr := 0
	if f.IncrementalEnabled {
		incr = 1
	}
	skip := 0
	if f.SkipIfUnchanged {
		skip = 1
	}
	_, err := s.DB.Exec(`
		INSERT INTO file_servers(
			id, name, protocol, host, port, username, remote_root, include_paths, exclude_paths, auth_mode, enc_secret,
			schedule_cron, schedule_start, retain_count, retain_days,
			retain_hourly, retain_daily, retain_weekly, retain_monthly, retain_yearly,
			incremental_enabled, skip_if_unchanged, ssh_host_key, enabled, created_at, updated_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, protocol=excluded.protocol, host=excluded.host, port=excluded.port,
			username=excluded.username, remote_root=excluded.remote_root,
			include_paths=excluded.include_paths, exclude_paths=excluded.exclude_paths,
			auth_mode=excluded.auth_mode,
			enc_secret=COALESCE(excluded.enc_secret, file_servers.enc_secret),
			schedule_cron=excluded.schedule_cron, schedule_start=excluded.schedule_start,
			retain_count=excluded.retain_count, retain_days=excluded.retain_days,
			retain_hourly=excluded.retain_hourly, retain_daily=excluded.retain_daily,
			retain_weekly=excluded.retain_weekly, retain_monthly=excluded.retain_monthly,
			retain_yearly=excluded.retain_yearly,
			incremental_enabled=excluded.incremental_enabled,
			skip_if_unchanged=excluded.skip_if_unchanged,
			ssh_host_key=CASE WHEN excluded.ssh_host_key != '' THEN excluded.ssh_host_key ELSE file_servers.ssh_host_key END,
			enabled=excluded.enabled, updated_at=excluded.updated_at
		`, f.ID, f.Name, f.Protocol, f.Host, f.Port, f.Username, f.RemoteRoot, encodePaths(f.IncludePaths), encodePaths(f.ExcludePaths), f.AuthMode, f.EncSecret,
		f.ScheduleCron, f.ScheduleStart, f.RetainCount, f.RetainDays,
		f.RetainHourly, f.RetainDaily, f.RetainWeekly, f.RetainMonthly, f.RetainYearly,
		incr, skip, f.SSHHostKey, en, f.CreatedAt, f.UpdatedAt)
	return err
}

func (s *Store) LastSucceededVersion(targetType, targetID string) (*Version, error) {
	versions, err := s.ListVersions(targetType, targetID)
	if err != nil {
		return nil, err
	}
	for _, v := range versions {
		if v.Status == "succeeded" {
			cp := v
			return &cp, nil
		}
	}
	return nil, nil
}

func (s *Store) SetFileServerSSHHostKey(id, fingerprint string) error {
	_, err := s.DB.Exec(`UPDATE file_servers SET ssh_host_key=?, updated_at=datetime('now') WHERE id=?`, fingerprint, id)
	return err
}

func (s *Store) DeleteFileServer(id string) error {
	_, err := s.DB.Exec(`DELETE FROM file_servers WHERE id = ?`, id)
	return err
}

const databaseCols = `id, name, mysql_host, mysql_port, mysql_db, mysql_user, enc_mysql_password, include_tables,
		       tunnel_mode, file_server_id, ssh_host, ssh_port, ssh_username, auth_mode, enc_ssh_secret,
		       schedule_cron, schedule_start, retain_count, retain_days,
		       retain_hourly, retain_daily, retain_weekly, retain_monthly, retain_yearly,
		       skip_if_unchanged, enabled, created_at, updated_at`

func scanDatabase(scan func(dest ...any) error) (*Database, error) {
	var d Database
	var enabled, skip int
	var tablesJSON string
	if err := scan(&d.ID, &d.Name, &d.MysqlHost, &d.MysqlPort, &d.MysqlDB, &d.MysqlUser, &d.EncMysqlPassword, &tablesJSON,
		&d.TunnelMode, &d.FileServerID, &d.SSHHost, &d.SSHPort, &d.SSHUsername, &d.AuthMode, &d.EncSSHSecret,
		&d.ScheduleCron, &d.ScheduleStart, &d.RetainCount, &d.RetainDays,
		&d.RetainHourly, &d.RetainDaily, &d.RetainWeekly, &d.RetainMonthly, &d.RetainYearly,
		&skip, &enabled, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return nil, err
	}
	d.Enabled = enabled == 1
	d.SkipIfUnchanged = skip == 1
	d.IncludeTables = decodePaths(tablesJSON)
	return &d, nil
}

func (s *Store) ListDatabases() ([]Database, error) {
	rows, err := s.DB.Query(`SELECT ` + databaseCols + ` FROM databases ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Database
	for rows.Next() {
		d, err := scanDatabase(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

func (s *Store) GetDatabase(id string) (*Database, error) {
	d, err := scanDatabase(s.DB.QueryRow(`SELECT `+databaseCols+` FROM databases WHERE id = ?`, id).Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (s *Store) UpsertDatabase(d *Database) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if d.CreatedAt == "" {
		d.CreatedAt = now
	}
	d.UpdatedAt = now
	en := 0
	if d.Enabled {
		en = 1
	}
	skip := 0
	if d.SkipIfUnchanged {
		skip = 1
	}
	_, err := s.DB.Exec(`
		INSERT INTO databases(
			id, name, mysql_host, mysql_port, mysql_db, mysql_user, enc_mysql_password, include_tables,
			tunnel_mode, file_server_id, ssh_host, ssh_port, ssh_username, auth_mode, enc_ssh_secret,
			schedule_cron, schedule_start, retain_count, retain_days,
			retain_hourly, retain_daily, retain_weekly, retain_monthly, retain_yearly,
			skip_if_unchanged, enabled, created_at, updated_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, mysql_host=excluded.mysql_host, mysql_port=excluded.mysql_port,
			mysql_db=excluded.mysql_db, mysql_user=excluded.mysql_user,
			enc_mysql_password=COALESCE(excluded.enc_mysql_password, databases.enc_mysql_password),
			include_tables=excluded.include_tables,
			tunnel_mode=excluded.tunnel_mode, file_server_id=excluded.file_server_id,
			ssh_host=excluded.ssh_host, ssh_port=excluded.ssh_port, ssh_username=excluded.ssh_username,
			auth_mode=excluded.auth_mode,
			enc_ssh_secret=COALESCE(excluded.enc_ssh_secret, databases.enc_ssh_secret),
			schedule_cron=excluded.schedule_cron, schedule_start=excluded.schedule_start,
			retain_count=excluded.retain_count, retain_days=excluded.retain_days,
			retain_hourly=excluded.retain_hourly, retain_daily=excluded.retain_daily,
			retain_weekly=excluded.retain_weekly, retain_monthly=excluded.retain_monthly,
			retain_yearly=excluded.retain_yearly,
			skip_if_unchanged=excluded.skip_if_unchanged,
			enabled=excluded.enabled, updated_at=excluded.updated_at
	`, d.ID, d.Name, d.MysqlHost, d.MysqlPort, d.MysqlDB, d.MysqlUser, d.EncMysqlPassword, encodePaths(d.IncludeTables),
		d.TunnelMode, nullStr(d.FileServerID), nullStr(d.SSHHost), d.SSHPort, nullStr(d.SSHUsername), d.AuthMode, d.EncSSHSecret,
		d.ScheduleCron, d.ScheduleStart, d.RetainCount, d.RetainDays,
		d.RetainHourly, d.RetainDaily, d.RetainWeekly, d.RetainMonthly, d.RetainYearly,
		skip, en, d.CreatedAt, d.UpdatedAt)
	return err
}

func (s *Store) DeleteDatabase(id string) error {
	_, err := s.DB.Exec(`DELETE FROM databases WHERE id = ?`, id)
	return err
}

func nullStr(ns sql.NullString) any {
	if ns.Valid {
		return ns.String
	}
	return nil
}
