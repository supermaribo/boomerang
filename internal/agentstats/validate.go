package agentstats

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	SSHActionExport = "export"
	SSHActionLogs   = "logs"

	DefaultLogLines = 200
	MaxLogLines     = 1000
)

// SSHAction is a validated forced-command invocation.
type SSHAction struct {
	Kind  string // SSHActionExport or SSHActionLogs
	Since time.Time
	Lines int
	Unit  string
}

// ParseSSHCommand validates SSH_ORIGINAL_COMMAND for the forced-command key.
// Only metrics export and journal log reads are allowed.
func ParseSSHCommand(cmd string) (SSHAction, error) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return SSHAction{}, fmt.Errorf("empty SSH command")
	}
	switch {
	case cmd == "boomerang-monitor ssh-export" || strings.HasPrefix(cmd, "boomerang-monitor ssh-export "):
		since, err := parseExportArgs(strings.TrimSpace(strings.TrimPrefix(cmd, "boomerang-monitor ssh-export")))
		if err != nil {
			return SSHAction{}, err
		}
		return SSHAction{Kind: SSHActionExport, Since: since}, nil
	case cmd == "boomerang-monitor ssh-logs" || strings.HasPrefix(cmd, "boomerang-monitor ssh-logs "):
		lines, unit, err := parseLogsArgs(strings.TrimSpace(strings.TrimPrefix(cmd, "boomerang-monitor ssh-logs")))
		if err != nil {
			return SSHAction{}, err
		}
		return SSHAction{Kind: SSHActionLogs, Lines: lines, Unit: unit}, nil
	default:
		return SSHAction{}, fmt.Errorf("forbidden SSH command")
	}
}

// ValidateSSHCommand is kept for callers that only need export; prefer ParseSSHCommand.
func ValidateSSHCommand(cmd string) (since time.Time, err error) {
	a, err := ParseSSHCommand(cmd)
	if err != nil {
		return time.Time{}, err
	}
	if a.Kind != SSHActionExport {
		return time.Time{}, fmt.Errorf("forbidden SSH command")
	}
	return a.Since, nil
}

func parseExportArgs(rest string) (time.Time, error) {
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

func parseLogsArgs(rest string) (lines int, unit string, err error) {
	lines = DefaultLogLines
	if rest == "" {
		return lines, "", nil
	}
	for _, part := range strings.Fields(rest) {
		switch {
		case strings.HasPrefix(part, "--lines="):
			raw := strings.Trim(strings.TrimPrefix(part, "--lines="), `"'`)
			n, e := strconv.Atoi(raw)
			if e != nil || n < 1 {
				return 0, "", fmt.Errorf("invalid --lines")
			}
			if n > MaxLogLines {
				n = MaxLogLines
			}
			lines = n
		case strings.HasPrefix(part, "--unit="):
			raw := strings.Trim(strings.TrimPrefix(part, "--unit="), `"'`)
			if raw == "" {
				continue
			}
			if !validUnitName(raw) {
				return 0, "", fmt.Errorf("invalid --unit")
			}
			unit = raw
		default:
			return 0, "", fmt.Errorf("forbidden SSH command")
		}
	}
	return lines, unit, nil
}

func validUnitName(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' || r == '@' {
			continue
		}
		return false
	}
	return true
}
