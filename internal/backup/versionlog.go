package backup

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const VersionLogFile = "backup.log"
const SkippedLogFile = "skipped.log"

// DefaultSkippedLogInline is how many missed paths to write into backup.log.
const DefaultSkippedLogInline = 250

// SkippedLogLines formats missed paths for human-readable backup logs.
func SkippedLogLines(paths []string, maxShow int) []string {
	if len(paths) == 0 {
		return nil
	}
	if maxShow <= 0 {
		maxShow = DefaultSkippedLogInline
	}
	lines := []string{fmt.Sprintf("--- missed paths (%d) ---", len(paths))}
	for i, p := range paths {
		if i >= maxShow {
			lines = append(lines, fmt.Sprintf("... and %d more not shown", len(paths)-maxShow))
			break
		}
		lines = append(lines, "missed: "+p)
	}
	return lines
}

// LogHasMissedPaths reports whether lines already include a missed-path section.
func LogHasMissedPaths(lines []string) bool {
	for _, line := range lines {
		if strings.HasPrefix(line, "--- missed paths") || strings.HasPrefix(line, "missed: ") {
			return true
		}
	}
	return false
}

// VersionLogger appends timestamped lines to backup.log in a version directory.
type VersionLogger struct {
	f *os.File
}

func NewVersionLogger(versionDir string) (*VersionLogger, error) {
	if err := os.MkdirAll(versionDir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(versionDir, VersionLogFile), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &VersionLogger{f: f}, nil
}

func (l *VersionLogger) Log(line string) {
	if l == nil || l.f == nil {
		return
	}
	line = strings.TrimRight(line, "\n")
	if line == "" {
		return
	}
	_, _ = fmt.Fprintln(l.f, line)
}

func (l *VersionLogger) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}

func ReadVersionLog(versionDir string) ([]string, error) {
	return readLogFile(filepath.Join(versionDir, VersionLogFile))
}

func ReadSkippedLog(versionDir string) ([]string, error) {
	return readLogFile(filepath.Join(versionDir, SkippedLogFile))
}

func readLogFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return lines, err
	}
	return lines, nil
}
