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
