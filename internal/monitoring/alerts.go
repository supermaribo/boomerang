package monitoring

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/boomerang-backup/boomerang/internal/store"
)

const alertCooldown = 6 * time.Hour

func (p *Poller) evaluateAlerts(m store.MonitoredServer) {
	if !m.AlertsEnabled {
		return
	}
	now := time.Now().UTC()
	offline := false
	offlineDetail := ""
	if !m.LastSampleAt.Valid {
		offline = true
		offlineDetail = "No metrics received yet"
	} else if t, err := store.ParseStoreTime(m.LastSampleAt.String); err == nil {
		grace := time.Duration(m.OfflineAfterSec) * time.Second
		if grace < time.Minute {
			grace = 3 * time.Minute
		}
		if now.Sub(t) > grace {
			offline = true
			offlineDetail = fmt.Sprintf("No metrics for %s", now.Sub(t).Truncate(time.Second))
		}
	} else {
		offline = true
		offlineDetail = "Invalid last sample timestamp"
	}
	p.setAlert(m, "offline", offline, offlineDetail, now)

	if offline {
		return
	}

	sample, fs, err := p.store.LatestMonitorSample(m.ID)
	if err != nil || sample == nil {
		return
	}
	sustain := time.Duration(m.AlertSustainSec) * time.Second
	if sustain < time.Minute {
		sustain = 5 * time.Minute
	}

	cpuHigh := sample.CPUPercent >= m.AlertCPUPercent && m.AlertCPUPercent > 0
	p.setSustained(m, "cpu", cpuHigh, fmt.Sprintf("%.0f%% CPU", sample.CPUPercent), sustain, now)

	memPct := 0.0
	if sample.MemTotalBytes > 0 {
		memPct = 100 * float64(sample.MemUsedBytes) / float64(sample.MemTotalBytes)
	}
	memHigh := memPct >= m.AlertMemPercent && m.AlertMemPercent > 0
	p.setSustained(m, "memory", memHigh, fmt.Sprintf("%.0f%% memory used", memPct), sustain, now)

	loadNorm := sample.Load1
	if sample.NumCPU > 0 {
		loadNorm = sample.Load1 / float64(sample.NumCPU)
	}
	loadHigh := loadNorm >= m.AlertLoadPerCPU && m.AlertLoadPerCPU > 0
	p.setSustained(m, "load", loadHigh, fmt.Sprintf("load1 %.2f (%.2f/CPU)", sample.Load1, loadNorm), sustain, now)

	diskHigh := false
	diskDetail := ""
	for _, f := range fs {
		if f.TotalBytes <= 0 {
			continue
		}
		pct := 100 * float64(f.UsedBytes) / float64(f.TotalBytes)
		if pct >= m.AlertDiskPercent && m.AlertDiskPercent > 0 {
			diskHigh = true
			diskDetail = fmt.Sprintf("%s at %.0f%%", f.Mount, pct)
			break
		}
	}
	p.setSustained(m, "disk", diskHigh, diskDetail, sustain, now)
}

func (p *Poller) setSustained(m store.MonitoredServer, key string, condition bool, detail string, sustain time.Duration, now time.Time) {
	st, _ := p.store.GetMonitorAlertState(m.ID, key+"_pending")
	if condition {
		if st == nil || !st.Active {
			_ = p.store.UpsertMonitorAlertState(store.MonitorAlertState{
				ServerID: m.ID, AlertKey: key + "_pending", Active: true,
				SinceAt: sql.NullString{String: now.Format(time.RFC3339), Valid: true},
				LastValue: detail,
			})
			return
		}
		since, err := store.ParseStoreTime(st.SinceAt.String)
		if err != nil || now.Sub(since) < sustain {
			_ = p.store.UpsertMonitorAlertState(store.MonitorAlertState{
				ServerID: m.ID, AlertKey: key + "_pending", Active: true,
				SinceAt: st.SinceAt, LastValue: detail,
			})
			return
		}
		p.setAlert(m, key, true, detail, now)
		return
	}
	if st != nil && st.Active {
		_ = p.store.UpsertMonitorAlertState(store.MonitorAlertState{
			ServerID: m.ID, AlertKey: key + "_pending", Active: false,
		})
	}
	p.setAlert(m, key, false, detail, now)
}

func (p *Poller) setAlert(m store.MonitoredServer, key string, active bool, detail string, now time.Time) {
	prev, _ := p.store.GetMonitorAlertState(m.ID, key)
	wasActive := prev != nil && prev.Active
	if active == wasActive {
		if active && prev != nil {
			_ = p.store.UpsertMonitorAlertState(store.MonitorAlertState{
				ServerID: m.ID, AlertKey: key, Active: true,
				SinceAt: prev.SinceAt, LastSentAt: prev.LastSentAt, LastValue: detail,
			})
		}
		return
	}

	state := store.MonitorAlertState{
		ServerID: m.ID, AlertKey: key, Active: active, LastValue: detail,
	}
	if active {
		state.SinceAt = sql.NullString{String: now.Format(time.RFC3339), Valid: true}
	} else if prev != nil {
		state.SinceAt = prev.SinceAt
	}

	shouldSend := true
	if prev != nil && prev.LastSentAt.Valid {
		if t, err := store.ParseStoreTime(prev.LastSentAt.String); err == nil && now.Sub(t) < alertCooldown && active {
			shouldSend = false
		}
	}
	if shouldSend {
		if err := p.sendAlertMail(m, key, active, detail); err != nil {
			log.Printf("monitor alert mail %s/%s: %v", m.Name, key, err)
		} else {
			state.LastSentAt = sql.NullString{String: now.Format(time.RFC3339), Valid: true}
		}
	} else if prev != nil {
		state.LastSentAt = prev.LastSentAt
	}
	_ = p.store.UpsertMonitorAlertState(state)
}

func (p *Poller) sendAlertMail(m store.MonitoredServer, key string, active bool, detail string) error {
	if p.notifyLoad == nil {
		return nil
	}
	cfg, err := p.notifyLoad()
	if err != nil || !cfg.Ready() {
		return nil
	}
	if !cfg.Alerts.MonitorFailure {
		return nil
	}
	label := alertLabel(key)
	var subject, body string
	if active {
		subject = fmt.Sprintf("[Boomerang] %s: %s", label, m.Name)
		body = fmt.Sprintf(`Boomerang monitoring alert.

Server:  %s (%s)
Alert:   %s
Detail:  %s
Time:    %s

Open Boomerang → Monitoring for history and thresholds.
`, m.Name, m.Host, label, detail, time.Now().UTC().Format(time.RFC3339))
	} else {
		subject = fmt.Sprintf("[Boomerang] Recovered %s: %s", label, m.Name)
		body = fmt.Sprintf(`Boomerang monitoring recovery.

Server:  %s (%s)
Alert:   %s cleared
Time:    %s
`, m.Name, m.Host, label, time.Now().UTC().Format(time.RFC3339))
	}
	return cfg.Send(subject, body)
}

func alertLabel(key string) string {
	switch key {
	case "offline":
		return "Server offline"
	case "cpu":
		return "High CPU"
	case "memory":
		return "High memory"
	case "disk":
		return "High disk usage"
	case "load":
		return "High load"
	default:
		return key
	}
}

// StatusFor returns a compact online/offline label for UI.
func StatusFor(m store.MonitoredServer, now time.Time) (online bool, detail string) {
	if !m.Enabled {
		return false, "Disabled"
	}
	if m.LastPollError != "" && (!m.LastSampleAt.Valid || isStale(m, now)) {
		return false, m.LastPollError
	}
	if !m.LastSampleAt.Valid {
		return false, "Waiting for first sample"
	}
	if isStale(m, now) {
		return false, "Offline"
	}
	return true, "Online"
}

func isStale(m store.MonitoredServer, now time.Time) bool {
	if !m.LastSampleAt.Valid {
		return true
	}
	t, err := store.ParseStoreTime(m.LastSampleAt.String)
	if err != nil {
		return true
	}
	grace := time.Duration(m.OfflineAfterSec) * time.Second
	if grace < time.Minute {
		grace = 3 * time.Minute
	}
	return now.Sub(t) > grace
}