package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/schedule"
	"github.com/boomerang-backup/boomerang/internal/tzutil"
)

type targetHealthRow struct {
	ID               string `json:"id"`
	TargetType       string `json:"targetType"`
	Name             string `json:"name"`
	Enabled          bool   `json:"enabled"`
	Scheduled        bool   `json:"scheduled"`
	LastSuccessAt    string `json:"lastSuccessAt,omitempty"`
	LastSuccessBytes int64  `json:"lastSuccessBytes,omitempty"`
	VersionCount     int    `json:"versionCount"`
	LastJobStatus    string `json:"lastJobStatus,omitempty"`
	LastJobError     string `json:"lastJobError,omitempty"`
	NextRunAt        string `json:"nextRunAt,omitempty"`
	Health           string `json:"health"`
	HealthDetail     string `json:"healthDetail,omitempty"`
}

func (s *Server) handleTargetHealth(w http.ResponseWriter, _ *http.Request) {
	var out []targetHealthRow

	files, _ := s.store.ListFileServers()
	for _, f := range files {
		out = append(out, s.healthForTarget("file", f.ID, f.Name, f.Enabled, f.ScheduleCron, f.ScheduleStart))
	}
	dbs, _ := s.store.ListDatabases()
	for _, d := range dbs {
		out = append(out, s.healthForTarget("db", d.ID, d.Name, d.Enabled, d.ScheduleCron, d.ScheduleStart))
	}
	writeJSON(w, http.StatusOK, map[string]any{"targets": out})
}

func (s *Server) healthForTarget(targetType, id, name string, enabled bool, cron, scheduleStart string) targetHealthRow {
	row := targetHealthRow{
		ID:         id,
		TargetType: targetType,
		Name:       name,
		Enabled:    enabled,
		Scheduled:  enabled && strings.TrimSpace(cron) != "",
		Health:     "idle",
	}
	if row.Scheduled {
		loc := tzutil.Load(s.store)
		if next, ok := schedule.NextRun(cron, loc, time.Now()); ok {
			row.NextRunAt = next.UTC().Format(time.RFC3339)
		}
	}
	if cnt, err := s.store.CountVersions(targetType, id); err == nil {
		row.VersionCount = cnt
	}
	if last, _ := s.store.LastSucceededVersion(targetType, id); last != nil {
		row.LastSuccessAt = last.CreatedAt
		row.LastSuccessBytes = last.Bytes
	}
	if job, _ := s.store.LastJobForTarget(targetType, id); job != nil {
		row.LastJobStatus = job.Status
		if job.Error != "" {
			row.LastJobError = job.Error
		}
	}
	if !row.Scheduled {
		row.HealthDetail = "No schedule configured"
		return row
	}
	if !scheduleActive(scheduleStart) {
		row.Health = "idle"
		row.HealthDetail = "Waiting for schedule start date"
		return row
	}
	if row.LastSuccessAt == "" {
		row.Health = "warning"
		row.HealthDetail = "Scheduled but no successful backup yet"
		return row
	}

	lastCheckAt := row.LastSuccessAt
	lastWasSkip := false
	if check, _ := s.store.LastBackupCheck(targetType, id); check != nil {
		when := check.CreatedAt
		if check.FinishedAt.Valid && check.FinishedAt.String != "" {
			when = check.FinishedAt.String
		}
		if when != "" {
			lastCheckAt = when
		}
		lastWasSkip = check.Status == "skipped"
	}

	if t, ok := parseHealthTime(lastCheckAt); ok {
		loc := tzutil.Load(s.store)
		if schedule.Overdue(cron, loc, t, time.Now().UTC()) {
			row.Health = "error"
			row.HealthDetail = "Missed a scheduled backup check"
			return row
		}
	}
	if row.LastJobStatus == "failed" {
		row.Health = "warning"
		row.HealthDetail = "Last job failed"
		return row
	}
	row.Health = "ok"
	if lastWasSkip {
		row.HealthDetail = "Checked on schedule (no changes)"
	} else {
		row.HealthDetail = "Backing up on schedule"
	}
	return row
}

func scheduleActive(start string) bool {
	start = strings.TrimSpace(start)
	if start == "" {
		return true
	}
	t, ok := parseHealthTime(start)
	if !ok {
		return true
	}
	return !time.Now().UTC().Before(t)
}

func parseHealthTime(s string) (time.Time, bool) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
