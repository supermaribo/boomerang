package agentstats

import (
	"testing"
	"time"
)

func TestValidateSSHCommand(t *testing.T) {
	cases := []struct {
		cmd     string
		wantErr bool
		since   string
	}{
		{"boomerang-monitor ssh-export", false, ""},
		{`boomerang-monitor ssh-export --since=2026-07-17T10:00:00Z`, false, "2026-07-17T10:00:00Z"},
		{"boomerang-monitor ssh-export --since=", false, ""},
		{"/bin/bash", true, ""},
		{"boomerang-monitor ssh-export; rm -rf /", true, ""},
		{"boomerang-monitor collect", true, ""},
		{"", true, ""},
	}
	for _, tc := range cases {
		since, err := ValidateSSHCommand(tc.cmd)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%q: expected error", tc.cmd)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", tc.cmd, err)
		}
		if tc.since == "" {
			if !since.IsZero() {
				t.Fatalf("%q: expected zero since, got %v", tc.cmd, since)
			}
			continue
		}
		want, _ := time.Parse(time.RFC3339, tc.since)
		if !since.Equal(want) {
			t.Fatalf("%q: since=%v want %v", tc.cmd, since, want)
		}
	}
}
