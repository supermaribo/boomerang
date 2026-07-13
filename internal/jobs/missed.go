package jobs

import (
	"log"
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/notify"
)

func (s *Scheduler) SetMissedNotifier(load func() (notify.MailConfig, error), nameFor func(targetType, targetID string) string) {
	s.notifyLoad = load
	s.notifyName = nameFor
}

func (s *Scheduler) startMissedLoop() {
	if s.notifyLoad == nil {
		return
	}
	go func() {
		tick := time.NewTicker(6 * time.Hour)
		defer tick.Stop()
		s.checkMissedBackups()
		for range tick.C {
			s.checkMissedBackups()
		}
	}()
}

func (s *Scheduler) checkMissedBackups() {
	cfg, err := s.notifyLoad()
	if err != nil || !cfg.Ready() || !cfg.Alerts.BackupFailure {
		return
	}
	cut := time.Now().UTC().Add(-36 * time.Hour)

	check := func(targetType, id, name, scheduleStart string) {
		if !scheduleActive(scheduleStart) {
			return
		}
		lastVer, _ := s.store.LastSucceededVersion(targetType, id)
		if lastVer == nil {
			key := "missed_alert:" + targetType + ":" + id
			if sent, _, _ := s.store.GetMeta(key); sent != "" {
				if sentAt, ok := parseStart(sent); ok && time.Since(sentAt) < 12*time.Hour {
					return
				}
			}
			subject := "[Boomerang] No backup yet: " + name
			body := "Scheduled backups are enabled for " + name + " but no successful backup has completed yet.\n\nCheck schedules, credentials, and job logs in Boomerang."
			if err := cfg.Send(subject, body); err != nil {
				log.Printf("missed backup alert %s: %v", name, err)
				return
			}
			_ = s.store.SetMeta(key, time.Now().UTC().Format(time.RFC3339))
			return
		}

		// A no-change skip still counts as a successful check — do not alert if one ran recently.
		if check, _ := s.store.LastBackupCheck(targetType, id); check != nil {
			when := check.CreatedAt
			if check.FinishedAt.Valid && check.FinishedAt.String != "" {
				when = check.FinishedAt.String
			}
			if t, ok := parseStart(when); ok && t.After(cut) {
				return
			}
		}
		if t, ok := parseStart(lastVer.CreatedAt); ok && t.After(cut) {
			return
		}

		key := "missed_alert:" + targetType + ":" + id
		if sent, _, _ := s.store.GetMeta(key); sent != "" {
			if sentAt, ok := parseStart(sent); ok && time.Since(sentAt) < 12*time.Hour {
				return
			}
		}
		subject := "[Boomerang] No recent backup: " + name
		body := "No successful backup check for " + name + " in the last 36 hours.\n\nCheck schedules and job logs in Boomerang."
		if err := cfg.Send(subject, body); err != nil {
			log.Printf("missed backup alert %s: %v", name, err)
			return
		}
		_ = s.store.SetMeta(key, time.Now().UTC().Format(time.RFC3339))
	}

	files, _ := s.store.ListFileServers()
	for _, f := range files {
		if f.Enabled && strings.TrimSpace(f.ScheduleCron) != "" {
			check("file", f.ID, f.Name, f.ScheduleStart)
		}
	}
	dbs, _ := s.store.ListDatabases()
	for _, d := range dbs {
		if d.Enabled && strings.TrimSpace(d.ScheduleCron) != "" {
			check("db", d.ID, d.Name, d.ScheduleStart)
		}
	}
}

func scheduleActive(start string) bool {
	start = strings.TrimSpace(start)
	if start == "" {
		return true
	}
	t, ok := parseStart(start)
	if !ok {
		return true
	}
	return !time.Now().UTC().Before(t)
}
