package rsyncbackup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reOpen = regexp.MustCompile(`failed to open "([^"]+)"`)
	reDir  = regexp.MustCompile(`opendir "([^"]+)"`)
)

func runRsync(cmd *exec.Cmd) (partial bool, output string, err error) {
	out, err := cmd.CombinedOutput()
	output = string(out)
	if err == nil {
		return false, output, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 23 {
		return true, output, nil
	}
	return false, output, fmt.Errorf("rsync: %w (%s)", err, truncateOutput(output, 1500))
}

func summarizeRsyncWarnings(output, remoteRoot string) string {
	totalFiles := strings.Count(output, "failed to open")
	totalDirs := strings.Count(output, "opendir")
	if totalFiles == 0 && totalDirs == 0 {
		return "warning: rsync exit 23 (partial transfer — some paths could not be read)"
	}
	samples := sampleDeniedPaths(output, remoteRoot, 8)
	msg := fmt.Sprintf(
		"warning: rsync skipped %d file(s) and %d folder(s) (permission denied on remote)",
		totalFiles, totalDirs,
	)
	if len(samples) > 0 {
		msg += " — e.g. " + strings.Join(samples, ", ")
	}
	remaining := totalFiles + totalDirs - len(samples)
	if remaining > 0 {
		msg += fmt.Sprintf(" (and %d more)", remaining)
	}
	msg += ". Grant the backup user read access on the server, or add excludes for paths you do not need."
	return msg
}

func sampleDeniedPaths(output, remoteRoot string, max int) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "Permission denied") {
			continue
		}
		var path string
		if m := reOpen.FindStringSubmatch(line); len(m) == 2 {
			path = shortenRemotePath(m[1], remoteRoot)
		} else if m := reDir.FindStringSubmatch(line); len(m) == 2 {
			path = shortenRemotePath(m[1], remoteRoot) + "/"
		} else {
			continue
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
		if len(out) >= max {
			break
		}
	}
	return out
}

func shortenRemotePath(full, root string) string {
	full = strings.TrimSpace(full)
	root = strings.TrimSuffix(strings.TrimSpace(root), "/")
	if root != "" && strings.HasPrefix(full, root+"/") {
		return strings.TrimPrefix(full, root+"/")
	}
	if i := strings.Index(full, "/htdocs/"); i >= 0 {
		return strings.TrimPrefix(full[i+len("/htdocs/"):], "/")
	}
	parts := strings.Split(strings.Trim(full, "/"), "/")
	if len(parts) > 3 {
		return strings.Join(parts[len(parts)-3:], "/")
	}
	return strings.TrimPrefix(full, "/")
}

func stagingFileCount(staging string) (int, error) {
	n := 0
	err := filepath.Walk(staging, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if fi.Mode().IsRegular() {
			n++
		}
		return nil
	})
	return n, err
}

func truncateOutput(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
