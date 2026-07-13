package store

import (
	"testing"
	"time"
)

func TestMissingRetentionBuckets(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC) // Monday week 29
	r := Retention{Weekly: 1, Monthly: 1, Yearly: 1}

	// No versions yet — need all configured tiers.
	missing := MissingRetentionBuckets(nil, r, now)
	if len(missing) != 3 {
		t.Fatalf("expected weekly/monthly/yearly, got %v", missing)
	}

	// Same-week backup covers weekly only.
	versions := []Version{{
		ID: "v1", Status: "succeeded",
		CreatedAt: now.Add(-2 * time.Hour).Format(time.RFC3339),
	}}
	missing = MissingRetentionBuckets(versions, r, now)
	if len(missing) != 0 {
		t.Fatalf("same week/month/year should cover all, got %v", missing)
	}

	// Last week only — weekly missing for current week; month/year still covered.
	lastWeek := now.AddDate(0, 0, -7)
	versions = []Version{{
		ID: "v2", Status: "succeeded",
		CreatedAt: lastWeek.Format(time.RFC3339),
	}}
	missing = MissingRetentionBuckets(versions, r, now)
	if len(missing) != 1 || missing[0] != "weekly" {
		t.Fatalf("expected [weekly], got %v", missing)
	}

	// Last year — need weekly, monthly, yearly for current period.
	lastYear := time.Date(2025, 7, 13, 10, 0, 0, 0, time.UTC)
	versions = []Version{{
		ID: "v3", Status: "succeeded",
		CreatedAt: lastYear.Format(time.RFC3339),
	}}
	missing = MissingRetentionBuckets(versions, r, now)
	want := map[string]bool{"weekly": true, "monthly": true, "yearly": true}
	if len(missing) != 3 {
		t.Fatalf("expected 3 missing, got %v", missing)
	}
	for _, m := range missing {
		if !want[m] {
			t.Fatalf("unexpected %s in %v", m, missing)
		}
	}

	// Skipped versions do not count.
	versions = []Version{{
		ID: "v4", Status: "skipped",
		CreatedAt: now.Format(time.RFC3339),
	}}
	missing = MissingRetentionBuckets(versions, r, now)
	if len(missing) != 3 {
		t.Fatalf("skipped should not fill buckets, got %v", missing)
	}

	// Zero retention — never force.
	missing = MissingRetentionBuckets(nil, Retention{}, now)
	if len(missing) != 0 {
		t.Fatalf("expected none, got %v", missing)
	}
}
