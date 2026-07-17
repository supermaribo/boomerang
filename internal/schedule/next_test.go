package schedule

import (
	"testing"
	"time"
)

func TestNextRun(t *testing.T) {
	loc := time.UTC
	next, ok := NextRun("0 3 * * *", loc, time.Date(2026, 7, 10, 2, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("expected ok")
	}
	if next.Hour() != 3 {
		t.Fatalf("expected 03:00, got %v", next)
	}
}

func TestOverdue(t *testing.T) {
	loc := time.UTC
	// Weekly schedule: Sundays 02:00. Last check finished Sun Jul 12 02:00:05.
	lastCheck := time.Date(2026, 7, 12, 2, 0, 5, 0, time.UTC)
	weekly := "0 2 * * 0"

	// Friday Jul 17: 5 days since the check but next run is Sun Jul 19 — healthy.
	if Overdue(weekly, loc, lastCheck, time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)) {
		t.Fatal("weekly target should not be overdue between runs")
	}
	// Mon Jul 20 15:00: Sunday run missed by >12h grace — overdue.
	if !Overdue(weekly, loc, lastCheck, time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC)) {
		t.Fatal("weekly target should be overdue after a missed run + grace")
	}

	// Daily schedule: never overdue before the 36h floor.
	daily := "0 2 * * *"
	lastDaily := time.Date(2026, 7, 16, 2, 0, 0, 0, time.UTC)
	if Overdue(daily, loc, lastDaily, time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)) {
		t.Fatal("daily target should not be overdue within 36h")
	}
	if !Overdue(daily, loc, lastDaily, time.Date(2026, 7, 18, 2, 0, 0, 0, time.UTC)) {
		t.Fatal("daily target should be overdue after two missed runs")
	}

	// Unparseable cron falls back to the 36h rule.
	if !Overdue("not a cron", loc, lastDaily, time.Date(2026, 7, 18, 2, 0, 0, 0, time.UTC)) {
		t.Fatal("bad cron should fall back to 36h rule")
	}
}
