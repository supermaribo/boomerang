package targethealth

import (
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/schedule"
	"github.com/boomerang-backup/boomerang/internal/store"
)

// Result is the shared health classification for a backup target.
type Result struct {
	ID               string
	TargetType       string
	Name             string
	Enabled          bool
	Scheduled        bool
	LastSuccessAt    string
	LastSuccessBytes int64
	LastCheckAt      string
	VersionCount     int
	LastJobStatus    string
	LastJobError     string
	NextRunAt        string
	Health           string // idle | ok | warning | error
	HealthDetail     string
	LastWasSkip      bool
}

// Evaluate classifies one file or database target.
func Evaluate(st *store.Store, targetType, id, name string, enabled bool, cronExpr, scheduleStart string, now time.Time, loc *time.Location) Result {
	row := Result{
		ID:         id,
		TargetType: targetType,
		Name:       name,
		Enabled:    enabled,
		Scheduled:  enabled && strings.TrimSpace(cronExpr) != "",
		Health:     "idle",
	}
	if row.Scheduled {
		if next, ok := schedule.NextRun(cronExpr, loc, now); ok {
			row.NextRunAt = next.UTC().Format(time.RFC3339)
		}
	}
	if cnt, err := st.CountVersions(targetType, id); err == nil {
		row.VersionCount = cnt
	}
	if last, _ := st.LastSucceededVersion(targetType, id); last != nil {
		row.LastSuccessAt = last.CreatedAt
		row.LastSuccessBytes = last.Bytes
	}
	if job, _ := st.LastJobForTarget(targetType, id); job != nil {
		row.LastJobStatus = job.Status
		if job.Error != "" {
			row.LastJobError = job.Error
		}
	}
	if !row.Scheduled {
		row.HealthDetail = "No schedule configured"
		return row
	}
	if !ScheduleActive(scheduleStart, now) {
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
	if check, _ := st.LastBackupCheck(targetType, id); check != nil {
		when := check.CreatedAt
		if check.FinishedAt.Valid && check.FinishedAt.String != "" {
			when = check.FinishedAt.String
		}
		if when != "" {
			lastCheckAt = when
		}
		row.LastWasSkip = check.Status == "skipped"
	}
	row.LastCheckAt = lastCheckAt

	if t, ok := ParseTime(lastCheckAt); ok {
		if schedule.Overdue(cronExpr, loc, t, now.UTC()) {
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
	if row.LastWasSkip {
		row.HealthDetail = "Checked on schedule (no changes)"
	} else {
		row.HealthDetail = "Backing up on schedule"
	}
	return row
}

// ScheduleActive reports whether scheduleStart has been reached.
func ScheduleActive(start string, now time.Time) bool {
	start = strings.TrimSpace(start)
	if start == "" {
		return true
	}
	t, ok := ParseTime(start)
	if !ok {
		return true
	}
	return !now.Before(t)
}

// ParseTime accepts common store/RFC3339 timestamps.
func ParseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.ParseInLocation(f, s, time.UTC); err == nil {
			return t, true
		}
	}
	if t, err := store.ParseStoreTime(s); err == nil {
		return t, true
	}
	return time.Time{}, false
}
