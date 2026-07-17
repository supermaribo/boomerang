package schedule

import (
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// NextRun returns the next fire time for a standard 5-field cron in loc.
func NextRun(expr string, loc *time.Location, after time.Time) (time.Time, bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return time.Time{}, false
	}
	if loc == nil {
		loc = time.UTC
	}
	sch, err := cronParser.Parse(expr)
	if err != nil {
		return time.Time{}, false
	}
	return sch.Next(after.In(loc)), true
}

const (
	// overdueGrace is how long past a scheduled fire time we wait before
	// declaring the run missed (covers long-running backups and clock drift).
	overdueGrace = 12 * time.Hour
	// overdueFloor keeps the legacy behavior of never alerting sooner than
	// 36h after the last successful check, regardless of schedule frequency.
	overdueFloor = 36 * time.Hour
)

// Overdue reports whether a scheduled target has missed a run. It is true only
// when a scheduled fire time has passed since lastCheck (plus a grace period)
// without a new check, and at least 36h have elapsed since lastCheck. This
// makes weekly/monthly schedules count as healthy between runs instead of
// tripping a fixed 36-hour window.
func Overdue(expr string, loc *time.Location, lastCheck, now time.Time) bool {
	if now.Sub(lastCheck) < overdueFloor {
		return false
	}
	next, ok := NextRun(expr, loc, lastCheck)
	if !ok {
		// Unparseable cron: fall back to the plain 36h rule (already exceeded).
		return true
	}
	return now.After(next.Add(overdueGrace))
}
