package store

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/boomerang-backup/boomerang/internal/backup"
)

type Job struct {
	ID         string         `json:"id"`
	TargetType string         `json:"targetType"`
	TargetID   string         `json:"targetId"`
	Kind       string         `json:"kind"`
	Status     string         `json:"status"`
	Error      string         `json:"error"`
	StartedAt  sql.NullString `json:"startedAt"`
	FinishedAt sql.NullString `json:"finishedAt"`
	CreatedAt  string         `json:"createdAt"`
}

type Version struct {
	ID         string `json:"id"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	Status     string `json:"status"`
	Bytes      int64  `json:"bytes"`
	PathOnDisk string `json:"pathOnDisk"`
	CreatedAt  string `json:"createdAt"`
}

func (s *Store) CreateJob(id, targetType, targetID, kind string) error {
	_, err := s.DB.Exec(`INSERT INTO jobs(id, target_type, target_id, kind, status) VALUES(?,?,?,?, 'queued')`,
		id, targetType, targetID, kind)
	return err
}

func (s *Store) UpdateJob(id, status, errMsg string, started time.Time, finished *time.Time) error {
	var start any
	if !started.IsZero() {
		start = started.Format(time.RFC3339)
	}
	var fin any
	if finished != nil {
		fin = finished.Format(time.RFC3339)
	}
	_, err := s.DB.Exec(`UPDATE jobs SET status=?, error=?, started_at=COALESCE(?, started_at), finished_at=COALESCE(?, finished_at) WHERE id=?`,
		status, errMsg, start, fin, id)
	return err
}

func (s *Store) AppendJobLog(jobID, line string) error {
	return s.AppendJobLogs(jobID, []string{line})
}

func (s *Store) AppendJobLogs(jobID string, lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO job_logs(job_id, line) VALUES(?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, line := range lines {
		if _, err := stmt.Exec(jobID, line); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetJob(id string) (*Job, error) {
	var j Job
	err := s.DB.QueryRow(`SELECT id, target_type, target_id, kind, status, error, started_at, finished_at, created_at FROM jobs WHERE id=?`, id).
		Scan(&j.ID, &j.TargetType, &j.TargetID, &j.Kind, &j.Status, &j.Error, &j.StartedAt, &j.FinishedAt, &j.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (s *Store) ListJobLogs(jobID string, limit, offset int) ([]string, int, error) {
	if limit <= 0 {
		limit = 2000
	}
	if limit > 5000 {
		limit = 5000
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM job_logs WHERE job_id=?`, jobID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.Query(`SELECT line FROM job_logs WHERE job_id=? ORDER BY id LIMIT ? OFFSET ?`, jobID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, 0, err
		}
		out = append(out, line)
	}
	return out, total, rows.Err()
}

func (s *Store) CreateVersion(id, targetType, targetID, path string) error {
	_, err := s.DB.Exec(`INSERT INTO backup_versions(id, target_type, target_id, status, path_on_disk) VALUES(?,?,?,?,?)`,
		id, targetType, targetID, "pending", path)
	return err
}

func (s *Store) UpdateVersion(id, status string, bytes int64) error {
	_, err := s.DB.Exec(`UPDATE backup_versions SET status=?, bytes=? WHERE id=?`, status, bytes, id)
	return err
}

func (s *Store) GetVersion(id string) (*Version, error) {
	var v Version
	err := s.DB.QueryRow(`SELECT id, target_type, target_id, status, bytes, path_on_disk, created_at FROM backup_versions WHERE id=?`, id).
		Scan(&v.ID, &v.TargetType, &v.TargetID, &v.Status, &v.Bytes, &v.PathOnDisk, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *Store) ListVersions(targetType, targetID string) ([]Version, error) {
	rows, err := s.DB.Query(`SELECT id, target_type, target_id, status, bytes, path_on_disk, created_at FROM backup_versions WHERE target_type=? AND target_id=? ORDER BY created_at DESC`,
		targetType, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.TargetType, &v.TargetID, &v.Status, &v.Bytes, &v.PathOnDisk, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) CountVersions(targetType, targetID string) (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM backup_versions WHERE target_type=? AND target_id=? AND status='succeeded'`, targetType, targetID).Scan(&n)
	return n, err
}

func (s *Store) LastJobForTarget(targetType, targetID string) (*Job, error) {
	var j Job
	err := s.DB.QueryRow(`
		SELECT id, target_type, target_id, kind, status, error, started_at, finished_at, created_at
		FROM jobs WHERE target_type=? AND target_id=? ORDER BY created_at DESC LIMIT 1`,
		targetType, targetID).
		Scan(&j.ID, &j.TargetType, &j.TargetID, &j.Kind, &j.Status, &j.Error, &j.StartedAt, &j.FinishedAt, &j.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// LastBackupCheck returns the most recent completed backup job (succeeded or skipped/no-change).
func (s *Store) LastBackupCheck(targetType, targetID string) (*Job, error) {
	var j Job
	err := s.DB.QueryRow(`
		SELECT id, target_type, target_id, kind, status, error, started_at, finished_at, created_at
		FROM jobs
		WHERE target_type=? AND target_id=? AND kind='backup' AND status IN ('succeeded','skipped')
		ORDER BY COALESCE(finished_at, created_at) DESC
		LIMIT 1`,
		targetType, targetID).
		Scan(&j.ID, &j.TargetType, &j.TargetID, &j.Kind, &j.Status, &j.Error, &j.StartedAt, &j.FinishedAt, &j.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// ErrVersionNotFound is returned when a backup version does not exist for the target.
var ErrVersionNotFound = fmt.Errorf("version not found")

// ErrVersionInUse is returned when a version cannot be deleted because newer incrementals depend on it.
type ErrVersionInUse struct {
	Count int
}

func (e ErrVersionInUse) Error() string {
	if e.Count == 1 {
		return "cannot delete: 1 newer incremental backup depends on this version — delete newer backups first"
	}
	return fmt.Sprintf("cannot delete: %d newer incremental backups depend on this version — delete newer backups first", e.Count)
}

// FileVersionsDependingOn lists succeeded versions whose incremental chain includes ancestorID.
func (s *Store) FileVersionsDependingOn(targetID, ancestorID string) ([]Version, error) {
	versions, err := s.ListVersions("file", targetID)
	if err != nil {
		return nil, err
	}
	var deps []Version
	for _, v := range versions {
		if v.ID == ancestorID || v.Status != "succeeded" {
			continue
		}
		ok, err := s.fileVersionDependsOn(v, ancestorID)
		if err != nil {
			return nil, err
		}
		if ok {
			deps = append(deps, v)
		}
	}
	return deps, nil
}

func (s *Store) fileVersionDependsOn(v Version, ancestorID string) (bool, error) {
	curID := v.ID
	seen := map[string]bool{}
	for curID != "" {
		if seen[curID] {
			return false, nil
		}
		seen[curID] = true
		ver, err := s.GetVersion(curID)
		if err != nil {
			return false, err
		}
		if ver == nil {
			return false, nil
		}
		m, err := backup.ReadFileManifest(ver.PathOnDisk)
		if err != nil || m.BaseVersionID == "" {
			return false, nil
		}
		if m.BaseVersionID == ancestorID {
			return true, nil
		}
		curID = m.BaseVersionID
	}
	return false, nil
}

// DiscardVersion removes a pending or failed version row and its files on disk.
func (s *Store) DiscardVersion(versionID string) error {
	v, err := s.GetVersion(versionID)
	if err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	if v.PathOnDisk != "" {
		_ = os.RemoveAll(v.PathOnDisk)
	}
	_, _ = s.DB.Exec(`DELETE FROM manifest_files WHERE version_id=?`, versionID)
	_, err = s.DB.Exec(`DELETE FROM backup_versions WHERE id=?`, versionID)
	return err
}

// DeleteVersion removes a single backup version and its files on disk.
func (s *Store) DeleteVersion(targetType, targetID, versionID string) error {
	v, err := s.GetVersion(versionID)
	if err != nil {
		return err
	}
	if v == nil || v.TargetType != targetType || v.TargetID != targetID {
		return ErrVersionNotFound
	}
	if v.Status == "pending" || v.Status == "running" {
		return fmt.Errorf("backup is still in progress")
	}
	if targetType == "file" {
		deps, err := s.FileVersionsDependingOn(targetID, versionID)
		if err != nil {
			return err
		}
		if len(deps) > 0 {
			return ErrVersionInUse{Count: len(deps)}
		}
	}
	if v.PathOnDisk != "" {
		_ = os.RemoveAll(v.PathOnDisk)
	}
	_, err = s.DB.Exec(`DELETE FROM backup_versions WHERE id=?`, versionID)
	return err
}

// Retention is GFS-style (grandfather-father-son) plus optional legacy count/days.
type Retention struct {
	Hourly  int
	Daily   int
	Weekly  int
	Monthly int
	Yearly  int
	Count   int // legacy: keep last N
	Days    int // legacy: keep within N days
}

func (s *Store) PruneVersions(targetType, targetID string, r Retention) error {
	versions, err := s.ListVersions(targetType, targetID)
	if err != nil {
		return err
	}
	gfs := r.Hourly > 0 || r.Daily > 0 || r.Weekly > 0 || r.Monthly > 0 || r.Yearly > 0
	legacy := r.Count > 0 || r.Days > 0
	if !gfs && !legacy {
		return nil
	}

	keep := map[string]bool{}
	if gfs {
		for id := range gfsKeep(versions, r) {
			keep[id] = true
		}
	} else {
		if r.Count > 0 {
			n := 0
			for _, v := range versions {
				if v.Status != "succeeded" {
					continue
				}
				if n < r.Count {
					keep[v.ID] = true
					n++
				}
			}
		}
		if r.Days > 0 {
			cut := time.Now().UTC().AddDate(0, 0, -r.Days)
			for _, v := range versions {
				t, ok := parseVersionTime(v.CreatedAt)
				if ok && t.After(cut) && v.Status == "succeeded" {
					keep[v.ID] = true
				}
			}
		}
	}

	if targetType == "file" {
		refs := make([]backup.VersionRef, 0, len(versions))
		for _, v := range versions {
			refs = append(refs, backup.VersionRef{ID: v.ID, PathOnDisk: v.PathOnDisk})
		}
		backup.ExpandIncrementalChain(refs, keep)
	}

	for _, v := range versions {
		if keep[v.ID] || v.Status == "pending" || v.Status == "running" {
			continue
		}
		if v.PathOnDisk != "" {
			_ = os.RemoveAll(v.PathOnDisk)
		}
		_, _ = s.DB.Exec(`DELETE FROM backup_versions WHERE id=?`, v.ID)
	}
	return nil
}

func gfsKeep(versions []Version, r Retention) map[string]bool {
	type item struct {
		v Version
		t time.Time
	}
	var items []item
	for _, v := range versions {
		if v.Status != "succeeded" {
			continue
		}
		t, ok := parseVersionTime(v.CreatedAt)
		if !ok {
			continue
		}
		items = append(items, item{v: v, t: t.UTC()})
	}
	// newest first (ListVersions already DESC, but be safe)
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].t.After(items[i].t) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	keep := map[string]bool{}
	keepBuckets := func(n int, keyFn func(time.Time) string) {
		if n <= 0 {
			return
		}
		seen := map[string]bool{}
		for _, it := range items {
			k := keyFn(it.t)
			if seen[k] {
				continue
			}
			seen[k] = true
			keep[it.v.ID] = true
			if len(seen) >= n {
				return
			}
		}
	}
	keepBuckets(r.Hourly, func(t time.Time) string { return t.Format("2006-01-02-15") })
	keepBuckets(r.Daily, func(t time.Time) string { return t.Format("2006-01-02") })
	keepBuckets(r.Weekly, func(t time.Time) string {
		y, w := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w)
	})
	keepBuckets(r.Monthly, func(t time.Time) string { return t.Format("2006-01") })
	keepBuckets(r.Yearly, func(t time.Time) string { return t.Format("2006") })
	return keep
}

func parseVersionTime(s string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02T15:04:05Z", s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// PruneJobLogs removes job log lines older than keepDays.
func (s *Store) PruneJobLogs(keepDays int) error {
	if keepDays <= 0 {
		return nil
	}
	cut := time.Now().UTC().AddDate(0, 0, -keepDays).Format(time.RFC3339)
	_, err := s.DB.Exec(`
		DELETE FROM job_logs WHERE job_id IN (
			SELECT id FROM jobs WHERE COALESCE(finished_at, created_at) < ?
		)`, cut)
	return err
}

// CleanupStaleVersions removes failed/pending backup dirs left on disk,
// and trims old no-change (skipped) check rows so they do not accumulate forever.
func (s *Store) CleanupStaleVersions() error {
	rows, err := s.DB.Query(`SELECT id, path_on_disk, status FROM backup_versions WHERE status IN ('failed','pending')`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, path, status string
		if err := rows.Scan(&id, &path, &status); err != nil {
			return err
		}
		if path != "" {
			_ = os.RemoveAll(path)
		}
		if status == "pending" {
			_, _ = s.DB.Exec(`DELETE FROM backup_versions WHERE id=?`, id)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// Keep skipped check markers for 14 days (dashboard history only; no files on disk).
	_, err = s.DB.Exec(`DELETE FROM backup_versions WHERE status='skipped' AND created_at < datetime('now', '-14 days')`)
	return err
}
