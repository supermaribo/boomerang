//go:build linux

package hoststats

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func readHost(dataDir string) Stats {
	var s Stats
	s.NumCPU = 0
	if raw, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(raw))
		if len(fields) > 0 {
			s.Load1, _ = strconv.ParseFloat(fields[0], 64)
		}
	}
	f, err := os.Open("/proc/meminfo")
	if err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "MemTotal:") {
				s.MemTotalBytes = parseKB(line)
			} else if strings.HasPrefix(line, "MemAvailable:") {
				s.MemAvailBytes = parseKB(line)
			}
		}
		_ = f.Close()
	}
	if dataDir == "" {
		dataDir = "/"
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &st); err == nil {
		s.DiskFreeBytes = st.Bavail * uint64(st.Bsize)
		s.DiskTotalBytes = st.Blocks * uint64(st.Bsize)
	}
	return s
}

func parseKB(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	n, _ := strconv.ParseUint(fields[1], 10, 64)
	return n * 1024
}
