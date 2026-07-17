//go:build linux

package agentstats

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/metrics"
)

type logFamily struct {
	id    string
	label string
	dir   string
}

var allowedLogFamilies = []logFamily{
	{id: "nginx", label: "Nginx", dir: "/var/log/nginx"},
	{id: "apache2", label: "Apache", dir: "/var/log/apache2"},
	{id: "httpd", label: "Apache (httpd)", dir: "/var/log/httpd"},
}

// AvailableLogSources returns only allowlisted logs readable by this account.
func AvailableLogSources() []metrics.LogSource {
	out := []metrics.LogSource{
		{ID: "journal", Label: "System journal", Kind: "journal"},
		{ID: "unit/boomerang-monitor.service", Label: "Boomerang Monitor service", Kind: "journal"},
	}
	for _, family := range allowedLogFamilies {
		entries, err := os.ReadDir(family.dir)
		if err != nil {
			continue
		}
		switch family.id {
		case "nginx":
			out = append(out, metrics.LogSource{ID: "unit/nginx.service", Label: "Nginx service", Kind: "journal"})
		case "apache2":
			out = append(out, metrics.LogSource{ID: "unit/apache2.service", Label: "Apache service", Kind: "journal"})
		case "httpd":
			out = append(out, metrics.LogSource{ID: "unit/httpd.service", Label: "Apache httpd service", Kind: "journal"})
		}
		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink != 0 ||
				!candidateLogName(entry.Name()) ||
				!validSourceToken(entry.Name()) {
				continue
			}
			path := filepath.Join(family.dir, entry.Name())
			f, err := os.Open(path)
			if err != nil {
				continue
			}
			info, statErr := f.Stat()
			_ = f.Close()
			if statErr != nil || !info.Mode().IsRegular() {
				continue
			}
			out = append(out, metrics.LogSource{
				ID:    "file/" + family.id + "/" + entry.Name(),
				Label: family.label + " — " + entry.Name(),
				Kind:  "file",
			})
		}
	}
	sort.SliceStable(out[2:], func(i, j int) bool {
		return out[i+2].Label < out[j+2].Label
	})
	return out
}

func candidateLogName(name string) bool {
	if name == "" || strings.Contains(name, "/") || strings.HasSuffix(name, ".gz") {
		return false
	}
	return strings.HasSuffix(name, ".log") ||
		strings.HasSuffix(name, ".log.1") ||
		strings.HasSuffix(name, "_log") ||
		strings.HasSuffix(name, "_log.1")
}

// ReadLogSource returns recent lines from one discovered allowlisted source.
func ReadLogSource(lines int, source string) (string, error) {
	if lines < 1 {
		lines = DefaultLogLines
	}
	if lines > MaxLogLines {
		lines = MaxLogLines
	}
	if source == "" || source == "journal" {
		return readJournal(lines, "")
	}
	if strings.HasPrefix(source, "unit/") {
		unit := strings.TrimPrefix(source, "unit/")
		if !validUnitName(unit) {
			return "", fmt.Errorf("invalid log source")
		}
		return readJournal(lines, unit)
	}
	path, err := resolveLogFile(source)
	if err != nil {
		return "", err
	}
	return tailFile(path, lines)
}

func validUnitName(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' || r == '@' {
			continue
		}
		return false
	}
	return true
}

func resolveLogFile(source string) (string, error) {
	parts := strings.Split(source, "/")
	if len(parts) != 3 || parts[0] != "file" || !candidateLogName(parts[2]) {
		return "", fmt.Errorf("unknown log source")
	}
	for _, family := range allowedLogFamilies {
		if family.id != parts[1] {
			continue
		}
		path := filepath.Join(family.dir, parts[2])
		if filepath.Dir(path) != family.dir {
			break
		}
		linkInfo, err := os.Lstat(path)
		if err != nil || linkInfo.Mode()&os.ModeSymlink != 0 || !linkInfo.Mode().IsRegular() {
			return "", fmt.Errorf("log source unavailable")
		}
		f, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("log source unavailable")
		}
		info, statErr := f.Stat()
		_ = f.Close()
		if statErr != nil || !info.Mode().IsRegular() {
			return "", fmt.Errorf("log source unavailable")
		}
		return path, nil
	}
	return "", fmt.Errorf("unknown log source")
}

func readJournal(lines int, unit string) (string, error) {
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

func tailFile(path string, lines int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open log: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("log source unavailable")
	}
	const maxRead int64 = 4 << 20
	start := info.Size() - maxRead
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.LimitReader(f, maxRead))
	if err != nil {
		return "", err
	}
	if start > 0 {
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			data = data[i+1:]
		}
	}
	data = bytes.TrimRight(data, "\r\n")
	if len(data) == 0 {
		return "", nil
	}
	all := bytes.Split(data, []byte{'\n'})
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return string(bytes.Join(all, []byte{'\n'})) + "\n", nil
}
