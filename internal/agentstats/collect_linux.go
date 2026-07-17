//go:build linux

package agentstats

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/boomerang-backup/boomerang/internal/metrics"
)

// Collect reads current host metrics from /proc and mount table.
func Collect(clientVersion string) (metrics.Sample, error) {
	now := time.Now().UTC()
	s := metrics.Sample{
		SchemaVersion: metrics.SchemaVersion,
		SampledAt:     now,
		ClientVersion: clientVersion,
		NumCPU:        runtimeNumCPU(),
	}

	if b, err := os.ReadFile("/proc/sys/kernel/random/boot_id"); err == nil {
		s.BootID = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(b))
		if len(fields) > 0 {
			if f, err := strconv.ParseFloat(fields[0], 64); err == nil {
				s.UptimeSec = int64(f)
			}
		}
	}
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(b))
		if len(fields) >= 3 {
			s.Load1, _ = strconv.ParseFloat(fields[0], 64)
			s.Load5, _ = strconv.ParseFloat(fields[1], 64)
			s.Load15, _ = strconv.ParseFloat(fields[2], 64)
		}
	}
	readMem(&s)
	s.CPUPercent = readCPUPercent()
	readDefaultRouteNet(&s)
	s.Filesystems = readFilesystems()
	return s, nil
}

func runtimeNumCPU() int {
	n := 0
	if f, err := os.Open("/proc/cpuinfo"); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "processor") {
				n++
			}
		}
		_ = f.Close()
	}
	if n < 1 {
		n = 1
	}
	return n
}

func readMem(s *metrics.Sample) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer f.Close()
	var memTotal, memAvail, swapTotal, swapFree uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			memTotal = parseKB(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			memAvail = parseKB(line)
		case strings.HasPrefix(line, "SwapTotal:"):
			swapTotal = parseKB(line)
		case strings.HasPrefix(line, "SwapFree:"):
			swapFree = parseKB(line)
		}
	}
	s.MemTotalBytes = memTotal
	s.MemAvailBytes = memAvail
	if memTotal > memAvail {
		s.MemUsedBytes = memTotal - memAvail
	}
	s.SwapTotalBytes = swapTotal
	if swapTotal > swapFree {
		s.SwapUsedBytes = swapTotal - swapFree
	}
}

func parseKB(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	n, _ := strconv.ParseUint(fields[1], 10, 64)
	return n * 1024
}

var lastCPU idleBusy

type idleBusy struct {
	idle, total uint64
	ok          bool
}

func readCPUPercent() float64 {
	idle, total, ok := readCPUTimes()
	if !ok {
		return 0
	}
	pct := 0.0
	if lastCPU.ok && total > lastCPU.total {
		dTotal := total - lastCPU.total
		dIdle := idle - lastCPU.idle
		if dTotal > 0 {
			pct = (1 - float64(dIdle)/float64(dTotal)) * 100
			if pct < 0 {
				pct = 0
			}
			if pct > 100 {
				pct = 100
			}
		}
	}
	lastCPU = idleBusy{idle: idle, total: total, ok: true}
	return pct
}

func readCPUTimes() (idle, total uint64, ok bool) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	line := strings.SplitN(string(b), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, false
	}
	var vals []uint64
	for _, f := range fields[1:] {
		n, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			break
		}
		vals = append(vals, n)
		total += n
	}
	if len(vals) < 4 {
		return 0, 0, false
	}
	idle = vals[3]
	if len(vals) > 4 {
		idle += vals[4] // iowait
	}
	return idle, total, true
}

var skipFSTypes = map[string]bool{
	"tmpfs": true, "devtmpfs": true, "devpts": true, "proc": true, "sysfs": true,
	"cgroup": true, "cgroup2": true, "overlay": true, "squashfs": true,
	"rpc_pipefs": true, "fusectl": true, "debugfs": true, "tracefs": true,
	"securityfs": true, "pstore": true, "bpf": true, "nsfs": true,
	"autofs": true, "mqueue": true, "hugetlbfs": true, "configfs": true,
}

func readFilesystems() []metrics.Filesystem {
	b, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []metrics.Filesystem
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		device, mount, fsType := fields[0], fields[1], fields[2]
		if skipFSTypes[fsType] {
			continue
		}
		if !strings.HasPrefix(device, "/") && !strings.HasPrefix(device, "/dev/") {
			continue
		}
		mount = unescapeMount(mount)
		if seen[mount] {
			continue
		}
		seen[mount] = true
		var st syscall.Statfs_t
		if err := syscall.Statfs(mount, &st); err != nil {
			continue
		}
		total := st.Blocks * uint64(st.Bsize)
		free := st.Bavail * uint64(st.Bsize)
		if total == 0 {
			continue
		}
		used := total - free
		if used > total {
			used = total
		}
		out = append(out, metrics.Filesystem{
			Mount: mount, Device: device, FSType: fsType,
			TotalBytes: total, UsedBytes: used, FreeBytes: free,
		})
	}
	return out
}

func unescapeMount(s string) string {
	s = strings.ReplaceAll(s, `\040`, " ")
	s = strings.ReplaceAll(s, `\011`, "\t")
	s = strings.ReplaceAll(s, `\012`, "\n")
	s = strings.ReplaceAll(s, `\134`, `\`)
	return s
}

// SpoolDir is the default metrics spool path.
const DefaultSpoolDir = "/var/lib/boomerang-monitor/spool"

// AppendSample writes one NDJSON sample and prunes entries older than 7 days.
func AppendSample(spoolDir string, sample metrics.Sample) error {
	if err := os.MkdirAll(spoolDir, 0o750); err != nil {
		return err
	}
	day := sample.SampledAt.UTC().Format("2006-01-02")
	path := filepath.Join(spoolDir, day+".ndjson")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := encodeSample(sample)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return pruneSpool(spoolDir, 7)
}

func pruneSpool(spoolDir string, keepDays int) error {
	cut := time.Now().UTC().AddDate(0, 0, -keepDays)
	entries, err := os.ReadDir(spoolDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".ndjson") {
			continue
		}
		day := strings.TrimSuffix(name, ".ndjson")
		t, err := time.Parse("2006-01-02", day)
		if err != nil {
			continue
		}
		if t.Before(cut) {
			_ = os.Remove(filepath.Join(spoolDir, name))
		}
	}
	return nil
}

// ReadSince returns samples with SampledAt strictly after since (UTC).
func ReadSince(spoolDir string, since time.Time) ([]metrics.Sample, error) {
	since = since.UTC()
	entries, err := os.ReadDir(spoolDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []metrics.Sample
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".ndjson") {
			continue
		}
		day := strings.TrimSuffix(name, ".ndjson")
		t, err := time.Parse("2006-01-02", day)
		if err != nil {
			continue
		}
		// Skip day files entirely before the since day.
		if t.Add(24 * time.Hour).Before(since) {
			continue
		}
		samples, err := readDayFile(filepath.Join(spoolDir, name))
		if err != nil {
			return nil, err
		}
		for _, s := range samples {
			if s.SampledAt.After(since) {
				out = append(out, s)
			}
		}
	}
	return out, nil
}

func readDayFile(path string) ([]metrics.Sample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []metrics.Sample
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		s, err := decodeSample([]byte(line))
		if err != nil {
			continue
		}
		out = append(out, s)
	}
	return out, sc.Err()
}
