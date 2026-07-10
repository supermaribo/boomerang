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
