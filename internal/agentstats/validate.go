package agentstats

import (
	"fmt"
	"strings"
	"time"
)

// ValidateSSHCommand returns the since cursor if cmd is a valid export invocation.
// Any other SSH original command is rejected so forced-command keys cannot run a shell.
func ValidateSSHCommand(cmd string) (since time.Time, err error) {
	cmd = strings.TrimSpace(cmd)
	const prefix = "boomerang-monitor ssh-export"
	if cmd == "" {
		return time.Time{}, fmt.Errorf("empty SSH command")
	}
	if cmd != prefix && !strings.HasPrefix(cmd, prefix+" ") {
		return time.Time{}, fmt.Errorf("forbidden SSH command")
	}
	if cmd == prefix {
		return time.Time{}, nil
	}
	rest := strings.TrimSpace(strings.TrimPrefix(cmd, prefix))
	if rest == "" {
		return time.Time{}, nil
	}
	if !strings.HasPrefix(rest, "--since=") {
		return time.Time{}, fmt.Errorf("forbidden SSH command")
	}
	raw := strings.TrimPrefix(rest, "--since=")
	raw = strings.Trim(raw, `"'`)
	if raw == "" {
		return time.Time{}, nil
	}
	// Reject anything after the timestamp (injection).
	if strings.ContainsAny(raw, " \t\n;&|`$") {
		return time.Time{}, fmt.Errorf("forbidden SSH command")
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		t, err = time.Parse(time.RFC3339, raw)
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --since: %w", err)
	}
	return t.UTC(), nil
}
