package store

import (
	"database/sql"
	"fmt"
	"os"
	"time"
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
	_, err := s.DB.Exec(`INSERT INTO job_logs(job_id, line) VALUES(?, ?)`, jobID, line)
	return err
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

func (s *Store) ListJobLogs(jobID string) ([]string, error) {
	rows, err := s.DB.Query(`SELECT line FROM job_logs WHERE job_id=? ORDER BY id`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	return out, rows.Err()
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

// Retention is GFS-style (grandfather-father-son) plus optional legacy count/days.
type Retention struct {
	Hourly int
	Daily  int
	Weekly int
	Yearly int
	Count  int // legacy: keep last N
	Days   int // legacy: keep within N days
}

func (s *Store) PruneVersions(targetType, targetID string, r Retention) error {
	versions, err := s.ListVersions(targetType, targetID)
	if err != nil {
		return err
	}
	gfs := r.Hourly > 0 || r.Daily > 0 || r.Weekly > 0 || r.Yearly > 0
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
