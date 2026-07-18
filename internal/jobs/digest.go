package jobs

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/monitoring"
	"github.com/boomerang-backup/boomerang/internal/offsite"
	"github.com/boomerang-backup/boomerang/internal/targethealth"
	"github.com/boomerang-backup/boomerang/internal/tzutil"
	"github.com/boomerang-backup/boomerang/internal/update"
)

func (s *Scheduler) runDailyDigest() {
	if s.notifyLoad == nil {
		return
	}
	cfg, err := s.notifyLoad()
	if err != nil || !cfg.Ready() || !cfg.Alerts.DailyDigest {
		return
	}
	loc := tzutil.Load(s.store)
	now := time.Now().In(loc)
	dayKey := "daily_digest:" + now.Format("2006-01-02")
	if v, ok, _ := s.store.GetMeta(dayKey); ok && v == "1" {
		return
	}

	body, interesting := s.buildDigestBody(now, loc)
	if !interesting {
		_ = s.store.SetMeta(dayKey, "1")
		return
	}
	subject := fmt.Sprintf("[Boomerang] Daily health digest — %s", now.Format("2 Jan 2006"))
	if err := cfg.Send(subject, body); err != nil {
		log.Printf("daily digest: %v", err)
		return
	}
	_ = s.store.SetMeta(dayKey, "1")
	log.Printf("daily digest sent for %s", now.Format("2006-01-02"))
}

func (s *Scheduler) buildDigestBody(now time.Time, loc *time.Location) (string, bool) {
	var b strings.Builder
	fmt.Fprintf(&b, "Boomerang daily health digest (%s)\n\n", now.Format(time.RFC3339))
	interesting := false

	var overdue, warnings, okCount int
	files, _ := s.store.ListFileServers()
	for _, f := range files {
		r := targethealth.Evaluate(s.store, "file", f.ID, f.Name, f.Enabled, f.ScheduleCron, f.ScheduleStart, now, loc)
		switch r.Health {
		case "error":
			overdue++
			interesting = true
			fmt.Fprintf(&b, "OVERDUE  website %s — %s\n", r.Name, r.HealthDetail)
		case "warning":
			warnings++
			interesting = true
			fmt.Fprintf(&b, "WARNING  website %s — %s\n", r.Name, r.HealthDetail)
		case "ok":
			okCount++
		}
	}
	dbs, _ := s.store.ListDatabases()
	for _, d := range dbs {
		r := targethealth.Evaluate(s.store, "db", d.ID, d.Name, d.Enabled, d.ScheduleCron, d.ScheduleStart, now, loc)
		switch r.Health {
		case "error":
			overdue++
			interesting = true
			fmt.Fprintf(&b, "OVERDUE  database %s — %s\n", r.Name, r.HealthDetail)
		case "warning":
			warnings++
			interesting = true
			fmt.Fprintf(&b, "WARNING  database %s — %s\n", r.Name, r.HealthDetail)
		case "ok":
			okCount++
		}
	}
	if overdue == 0 && warnings == 0 {
		fmt.Fprintf(&b, "Backup targets: %d healthy on schedule.\n", okCount)
	}

	since := now.UTC().Add(-24 * time.Hour)
	failed, _ := s.store.ListFailedJobsSince(since)
	if len(failed) > 0 {
		interesting = true
		fmt.Fprintf(&b, "\nFailed jobs (last 24h): %d\n", len(failed))
		for i, j := range failed {
			if i >= 15 {
				fmt.Fprintf(&b, "  …and %d more\n", len(failed)-15)
				break
			}
			name := j.TargetID
			if s.notifyName != nil {
				name = s.notifyName(j.TargetType, j.TargetID)
			}
			errMsg := j.Error
			if errMsg == "" {
				errMsg = j.Status
			}
			fmt.Fprintf(&b, "  • %s (%s) %s — %s\n", name, j.Kind, j.CreatedAt, errMsg)
		}
	} else {
		b.WriteString("\nFailed jobs (last 24h): none\n")
	}

	st := offsite.LoadStatus(s.store)
	offsiteOn := false
	if v, ok, _ := s.store.GetMeta("offsite_enabled"); ok {
		offsiteOn = v == "1" || strings.EqualFold(v, "true")
	}
	if offsiteOn {
		if st.LastError != "" {
			interesting = true
			fmt.Fprintf(&b, "\nOff-site: ERROR — %s\n", st.LastError)
		} else if st.LastSync == "" {
			interesting = true
			b.WriteString("\nOff-site: enabled but no successful sync yet\n")
		} else {
			fmt.Fprintf(&b, "\nOff-site: last sync %s\n", st.LastSync)
		}
	} else {
		b.WriteString("\nOff-site: disabled\n")
	}

	monitors, _ := s.store.ListMonitoredServers()
	offline := 0
	updates := 0
	latest, _ := update.LatestTagCached()
	for _, m := range monitors {
		if !m.Enabled {
			continue
		}
		online, _ := monitoring.StatusFor(m, now.UTC())
		if !online {
			offline++
			interesting = true
			fmt.Fprintf(&b, "OFFLINE  monitor %s (%s)\n", m.Name, m.Host)
		}
		if latest != "" && update.ClientUpdateAvailable(m.ClientVersion, latest) {
			updates++
		}
	}
	if offline == 0 && len(monitors) > 0 {
		fmt.Fprintf(&b, "\nMonitors: all online (%d)\n", len(monitors))
	}
	if updates > 0 {
		interesting = true
		fmt.Fprintf(&b, "Monitor agents with update available: %d (latest %s)\n", updates, latest)
	}

	b.WriteString("\nOpen Boomerang for details.\n")
	return b.String(), interesting
}
