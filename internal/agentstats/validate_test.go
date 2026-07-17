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
		source  string
	}{
		{"boomerang-monitor ssh-export", false, SSHActionExport, "", 0, ""},
		{`boomerang-monitor ssh-export --since=2026-07-17T10:00:00Z`, false, SSHActionExport, "2026-07-17T10:00:00Z", 0, ""},
		{"boomerang-monitor ssh-export --since=", false, SSHActionExport, "", 0, ""},
		{"boomerang-monitor ssh-logs", false, SSHActionLogs, "", DefaultLogLines, "journal"},
		{"boomerang-monitor ssh-logs --lines=50", false, SSHActionLogs, "", 50, "journal"},
		{"boomerang-monitor ssh-logs --lines=50 --source=unit/sshd.service", false, SSHActionLogs, "", 50, "unit/sshd.service"},
		{"boomerang-monitor ssh-logs --source=file/nginx/access.log", false, SSHActionLogs, "", DefaultLogLines, "file/nginx/access.log"},
		{"boomerang-monitor ssh-log-sources", false, SSHActionLogSources, "", 0, ""},
		{"/bin/bash", true, "", "", 0, ""},
		{"boomerang-monitor ssh-export; rm -rf /", true, "", "", 0, ""},
		{"boomerang-monitor collect", true, "", "", 0, ""},
		{"boomerang-monitor ssh-log-sources --all", true, "", "", 0, ""},
		{"boomerang-monitor ssh-logs --source=../../etc/passwd", true, "", "", 0, ""},
		{"boomerang-monitor ssh-logs --source=file/nginx/../error.log", true, "", "", 0, ""},
		{"boomerang-monitor ssh-logs --source=file/other/access.log", true, "", "", 0, ""},
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
			if a.Source != tc.source {
				t.Fatalf("%q: source=%q want %q", tc.cmd, a.Source, tc.source)
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

func TestValidSourceID(t *testing.T) {
	for _, source := range []string{
		"journal",
		"unit/nginx.service",
		"file/nginx/access.log",
		"file/apache2/example-access.log",
		"file/httpd/error_log",
	} {
		if !validSourceID(source) {
			t.Errorf("expected valid source %q", source)
		}
	}
	for _, source := range []string{
		"../../etc/passwd",
		"file/nginx/../error.log",
		"file/other/access.log",
		"file/nginx",
		"/etc/passwd",
		"unit/a/b",
	} {
		if validSourceID(source) {
			t.Errorf("expected invalid source %q", source)
		}
	}
}
