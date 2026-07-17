package monitoring

import (
	"testing"
	"time"

	"github.com/boomerang-backup/boomerang/internal/store"
)

func TestStatusFor(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	m := store.MonitoredServer{
		Enabled:         true,
		OfflineAfterSec: 180,
		LastSampleAt:    store.FormatNullTime(ptrTime(now.Add(-time.Minute))),
	}
	online, _ := StatusFor(m, now)
	if !online {
		t.Fatal("expected online")
	}
	m.LastSampleAt = store.FormatNullTime(ptrTime(now.Add(-10 * time.Minute)))
	online, detail := StatusFor(m, now)
	if online || detail != "Offline" {
		t.Fatalf("expected offline, got %v %q", online, detail)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
