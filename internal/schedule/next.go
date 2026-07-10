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
