package hoststats

import "runtime"

// Stats holds basic host metrics for the dashboard.
type Stats struct {
	MemTotalBytes uint64
	MemAvailBytes uint64
	Load1         float64
	DiskFreeBytes uint64
	DiskTotalBytes uint64
	NumCPU        int
	OK            bool
}

func Collect(dataDir string) Stats {
	s := readHost(dataDir)
	if s.NumCPU < 1 {
		s.NumCPU = runtime.NumCPU()
	}
	s.OK = true
	return s
}
