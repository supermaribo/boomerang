package api

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/boomerang-backup/boomerang/internal/hoststats"
	"github.com/boomerang-backup/boomerang/internal/offsite"
)

type statusItem struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

func (s *Server) applianceStatus() []statusItem {
	var out []statusItem

	dbOK := s.store.DB.Ping() == nil
	out = append(out, statusItem{
		ID: "database", Label: "Database", OK: dbOK,
		Detail: detailOK(dbOK, "SQLite reachable", "database unreachable"),
	})

	masterPath := filepath.Join(s.cfg.DataDir, "secrets", "master.key")
	mkOK := false
	if st, err := os.Stat(masterPath); err == nil && st.Size() > 0 {
		mkOK = true
	}
	out = append(out, statusItem{
		ID: "masterKey", Label: "Master key", OK: mkOK,
		Detail: detailOK(mkOK, "Present on disk", "secrets/master.key missing"),
	})

	host := hoststats.Collect(s.cfg.DataDir)
	cpus := host.NumCPU
	if cpus < 1 {
		cpus = runtime.NumCPU()
	}
	loadOK := host.Load1 <= float64(cpus)*1.25+0.5
	loadDetail := "n/a"
	if host.Load1 > 0 || host.MemTotalBytes > 0 {
		loadDetail = fmt.Sprintf("Load %.2f (%d CPU)", host.Load1, cpus)
	}
	out = append(out, statusItem{
		ID: "cpu", Label: "CPU", OK: loadOK, Detail: loadDetail,
	})

	memOK := true
	memDetail := "n/a"
	if host.MemTotalBytes > 0 {
		pct := float64(host.MemAvailBytes) / float64(host.MemTotalBytes) * 100
		memOK = host.MemAvailBytes >= 128*1024*1024 && pct >= 8
		memDetail = fmt.Sprintf("%.0f%% free (%s / %s)", pct, fmtBytes(host.MemAvailBytes), fmtBytes(host.MemTotalBytes))
	}
	out = append(out, statusItem{
		ID: "memory", Label: "Memory", OK: memOK, Detail: memDetail,
	})

	diskOK := true
	diskDetail := "n/a"
	if host.DiskTotalBytes > 0 {
		pct := float64(host.DiskFreeBytes) / float64(host.DiskTotalBytes) * 100
		diskOK = host.DiskFreeBytes >= 512*1024*1024 && pct >= 5
		diskDetail = fmt.Sprintf("%.0f%% free (%s / %s)", pct, fmtBytes(host.DiskFreeBytes), fmtBytes(host.DiskTotalBytes))
	}
	out = append(out, statusItem{
		ID: "disk", Label: "Disk", OK: diskOK, Detail: diskDetail,
	})

	if s.runner != nil {
		running, queued := s.runner.Stats()
		jobsOK := queued < 100
		out = append(out, statusItem{
			ID: "jobs", Label: "Backup jobs", OK: jobsOK,
			Detail: fmt.Sprintf("%d running, %d queued", running, queued),
		})
	}

	cfg, _ := offsite.LoadConfig(s.store, s.box)
	if cfg.Enabled {
		st := offsite.LoadStatus(s.store)
		syncing := st.Syncing
		if s.offsite != nil && s.offsite.IsSyncing() {
			syncing = true
		}
		offOK := st.LastError == "" && !syncing
		detail := "Enabled"
		if syncing {
			detail = "Syncing to R2…"
			offOK = true
		} else if st.LastError != "" {
			detail = st.LastError
		} else if st.LastSync != "" {
			detail = "Last mirror " + formatStatusTime(st.LastSync)
		} else {
			detail = "No mirror yet"
			offOK = false
		}
		out = append(out, statusItem{
			ID: "offsite", Label: "Off-site (R2)", OK: offOK, Detail: detail,
		})
	}

	return out
}

func detailOK(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

func fmtBytes(n uint64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	if n < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
	return fmt.Sprintf("%.2f GB", float64(n)/(1024*1024*1024))
}

func formatStatusTime(raw string) string {
	layouts := []string{time.RFC3339, "2006-01-02T15:04:05Z"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Local().Format("2 Jan 2006, 15:04")
		}
	}
	return raw
}
