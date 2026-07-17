package agentstats

import (
	"testing"
	"time"
)

func TestParseSSHCommand(t *testing.T) {
	cases := []struct {
		cmd     string
		wantErr bool
		kind    string
		since   string
		lines   int
		unit    string
	}{
		{"boomerang-monitor ssh-export", false, SSHActionExport, "", 0, ""},
		{`boomerang-monitor ssh-export --since=2026-07-17T10:00:00Z`, false, SSHActionExport, "2026-07-17T10:00:00Z", 0, ""},
		{"boomerang-monitor ssh-export --since=", false, SSHActionExport, "", 0, ""},
		{"boomerang-monitor ssh-logs", false, SSHActionLogs, "", DefaultLogLines, ""},
		{"boomerang-monitor ssh-logs --lines=50", false, SSHActionLogs, "", 50, ""},
		{"boomerang-monitor ssh-logs --lines=50 --unit=sshd.service", false, SSHActionLogs, "", 50, "sshd.service"},
		{"boomerang-monitor ssh-logs --unit=boomerang-monitor.service", false, SSHActionLogs, "", DefaultLogLines, "boomerang-monitor.service"},
		{"/bin/bash", true, "", "", 0, ""},
		{"boomerang-monitor ssh-export; rm -rf /", true, "", "", 0, ""},
		{"boomerang-monitor collect", true, "", "", 0, ""},
		{"boomerang-monitor ssh-logs --unit=../../etc/passwd", true, "", "", 0, ""},
		{"boomerang-monitor ssh-logs --lines=abc", true, "", "", 0, ""},
		{"", true, "", "", 0, ""},
	}
	for _, tc := range cases {
		a, err := ParseSSHCommand(tc.cmd)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%q: expected error", tc.cmd)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", tc.cmd, err)
		}
		if a.Kind != tc.kind {
			t.Fatalf("%q: kind=%q want %q", tc.cmd, a.Kind, tc.kind)
		}
		if tc.kind == SSHActionExport {
			if tc.since == "" {
				if !a.Since.IsZero() {
					t.Fatalf("%q: expected zero since, got %v", tc.cmd, a.Since)
				}
			} else {
				want, _ := time.Parse(time.RFC3339, tc.since)
				if !a.Since.Equal(want) {
					t.Fatalf("%q: since=%v want %v", tc.cmd, a.Since, want)
				}
			}
		}
		if tc.kind == SSHActionLogs {
			if a.Lines != tc.lines {
				t.Fatalf("%q: lines=%d want %d", tc.cmd, a.Lines, tc.lines)
			}
			if a.Unit != tc.unit {
				t.Fatalf("%q: unit=%q want %q", tc.cmd, a.Unit, tc.unit)
			}
		}
	}
}

func TestValidateSSHCommandExportOnly(t *testing.T) {
	_, err := ValidateSSHCommand("boomerang-monitor ssh-logs")
	if err == nil {
		t.Fatal("expected error for logs via ValidateSSHCommand")
	}
}
