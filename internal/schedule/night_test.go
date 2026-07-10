package schedule

import (
	"strings"
	"testing"
)

func TestRandomNight(t *testing.T) {
	seen := make(map[int]bool)
	for i := 0; i < 200; i++ {
		cron, start := RandomNight()
		parts := strings.Fields(cron)
		if len(parts) != 5 || parts[2] != "*" || parts[3] != "*" || parts[4] != "*" {
			t.Fatalf("unexpected cron %q", cron)
		}
		h := atoi(parts[1])
		if h < 0 || h > 23 {
			t.Fatalf("hour out of range: %d in %q", h, cron)
		}
		if !isNightHour(h) {
			t.Fatalf("hour %d not in night window: %q", h, cron)
		}
		seen[h] = true
		if start == "" {
			t.Fatal("empty start time")
		}
	}
	if len(seen) < 3 {
		t.Fatalf("expected varied hours, got %d distinct", len(seen))
	}
}

func isNightHour(h int) bool {
	for _, nh := range nightHours {
		if nh == h {
			return true
		}
	}
	return false
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
