//go:build linux

package agentstats

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

// ReadJournal returns recent journald lines for the host (or a specific unit).
func ReadJournal(lines int, unit string) (string, error) {
	if lines < 1 {
		lines = DefaultLogLines
	}
	if lines > MaxLogLines {
		lines = MaxLogLines
	}
	if unit != "" && !validUnitName(unit) {
		return "", fmt.Errorf("invalid unit name")
	}

	args := []string{
		"--no-pager",
		"-o", "short-iso",
		"-n", strconv.Itoa(lines),
	}
	if unit != "" {
		args = append(args, "-u", unit)
	}

	cmd := exec.Command("journalctl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()
	select {
	case err := <-done:
		if err != nil {
			msg := stderr.String()
			if msg == "" {
				msg = err.Error()
			}
			return "", fmt.Errorf("journalctl: %s", msg)
		}
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("journalctl timed out")
	}
	return stdout.String(), nil
}
