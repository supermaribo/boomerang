package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/boomerang-backup/boomerang/internal/metrics"
)

type MonitoredServer struct {
	ID               string
	Name             string
	Host             string
	Port             int
	Username         string
	EncSecret        []byte
	SSHHostKey       string
	FileServerID     sql.NullString
	Enabled          bool
	PollIntervalSec  int
	OfflineAfterSec  int
	AlertCPUPercent  float64
	AlertMemPercent  float64
	AlertDiskPercent float64
	AlertLoadPerCPU  float64
	AlertSustainSec  int
	AlertsEnabled    bool
	ClientVersion    string
	LastSampleAt     sql.NullString
	LastPollAt       sql.NullString
	LastPollError    string
	LastBootID       string
	CreatedAt        string
	UpdatedAt        string
}

type MonitorSampleRow struct {
	ID             int64    `json:"id"`
	ServerID       string   `json:"serverId"`
	SampledAt      string   `json:"sampledAt"`
	BootID         string   `json:"bootId,omitempty"`
	UptimeSec      int64    `json:"uptimeSec"`
	CPUPercent     float64  `json:"cpuPercent"`
	MemTotalBytes  int64    `json:"memTotalBytes"`
	MemUsedBytes   int64    `json:"memUsedBytes"`
	MemAvailBytes  int64    `json:"memAvailBytes"`
	SwapTotalBytes int64    `json:"swapTotalBytes"`
	SwapUsedBytes  int64    `json:"swapUsedBytes"`
	Load1          float64  `json:"load1"`
	Load5          float64  `json:"load5"`
	Load15         float64  `json:"load15"`
	NumCPU         int      `json:"numCpu"`
	ClientVersion  string   `json:"clientVersion,omitempty"`
	NetIface       string   `json:"netIface,omitempty"`
	NetRxBytes     int64    `json:"netRxBytes"`
	NetTxBytes     int64    `json:"netTxBytes"`
	NetRxBps       *float64 `json:"netRxBps"`
	NetTxBps       *float64 `json:"netTxBps"`
}

type MonitorFSRow struct {
	Mount      string `json:"mount"`
	Device     string `json:"device,omitempty"`
	FSType     string `json:"fsType,omitempty"`
	TotalBytes int64  `json:"totalBytes"`
	UsedBytes  int64  `json:"usedBytes"`
	FreeBytes  int64  `json:"freeBytes"`
}

type MonitorHourlyRow struct {
	HourAt         string
	Samples        int
	AvgCPUPercent  float64
	MaxCPUPercent  float64
	AvgMemPercent  float64
	MaxMemPercent  float64
	AvgLoad1       float64
	MaxLoad1       float64
	MaxDiskPercent float64
	AvgNetRxBps    float64
	MaxNetRxBps    float64
	AvgNetTxBps    float64
	MaxNetTxBps    float64
}

type MonitorAlertState struct {
	ServerID   string
	AlertKey   string
	Active     bool
	SinceAt    sql.NullString
	LastSentAt sql.NullString
	LastValue  string
}

func (s *Store) ListMonitoredServers() ([]MonitoredServer, error) {
	rows, err := s.DB.Query(`
		SELECT id, name, host, port, username, enc_secret, ssh_host_key, file_server_id, enabled,
		       poll_interval_sec, offline_after_sec, alert_cpu_percent, alert_mem_percent, alert_disk_percent,
		       alert_load_per_cpu, alert_sustain_sec, alerts_enabled, client_version,
		       last_sample_at, last_poll_at, last_poll_error, last_boot_id, created_at, updated_at
		FROM monitored_servers ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MonitoredServer
	for rows.Next() {
		m, err := scanMonitoredServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) GetMonitoredServer(id string) (*MonitoredServer, error) {
	row := s.DB.QueryRow(`
		SELECT id, name, host, port, username, enc_secret, ssh_host_key, file_server_id, enabled,
		       poll_interval_sec, offline_after_sec, alert_cpu_percent, alert_mem_percent, alert_disk_percent,
		       alert_load_per_cpu, alert_sustain_sec, alerts_enabled, client_version,
		       last_sample_at, last_poll_at, last_poll_error, last_boot_id, created_at, updated_at
		FROM monitored_servers WHERE id=?`, id)
	m, err := scanMonitoredServer(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanMonitoredServer(row scannable) (MonitoredServer, error) {
	var m MonitoredServer
	var enabled, alerts int
	err := row.Scan(
		&m.ID, &m.Name, &m.Host, &m.Port, &m.Username, &m.EncSecret, &m.SSHHostKey, &m.FileServerID, &enabled,
		&m.PollIntervalSec, &m.OfflineAfterSec, &m.AlertCPUPercent, &m.AlertMemPercent, &m.AlertDiskPercent,
		&m.AlertLoadPerCPU, &m.AlertSustainSec, &alerts, &m.ClientVersion,
		&m.LastSampleAt, &m.LastPollAt, &m.LastPollError, &m.LastBootID, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return m, err
	}
	m.Enabled = enabled != 0
	m.AlertsEnabled = alerts != 0
	return m, nil
}

func (s *Store) UpsertMonitoredServer(m *MonitoredServer) error {
	if m.Port <= 0 {
		m.Port = 22
	}
	if m.PollIntervalSec <= 0 {
		m.PollIntervalSec = 60
	}
	if m.OfflineAfterSec <= 0 {
		m.OfflineAfterSec = 180
	}
	if m.AlertSustainSec <= 0 {
		m.AlertSustainSec = 300
	}
	enabled, alerts := 0, 0
	if m.Enabled {
		enabled = 1
	}
	if m.AlertsEnabled {
		alerts = 1
	}
	_, err := s.DB.Exec(`
		INSERT INTO monitored_servers(
			id, name, host, port, username, enc_secret, ssh_host_key, file_server_id, enabled,
			poll_interval_sec, offline_after_sec, alert_cpu_percent, alert_mem_percent, alert_disk_percent,
			alert_load_per_cpu, alert_sustain_sec, alerts_enabled, client_version,
			last_sample_at, last_poll_at, last_poll_error, last_boot_id, created_at, updated_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,COALESCE((SELECT created_at FROM monitored_servers WHERE id=?), datetime('now')), datetime('now'))
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, host=excluded.host, port=excluded.port, username=excluded.username,
			enc_secret=excluded.enc_secret, ssh_host_key=excluded.ssh_host_key, file_server_id=excluded.file_server_id,
			enabled=excluded.enabled, poll_interval_sec=excluded.poll_interval_sec, offline_after_sec=excluded.offline_after_sec,
			alert_cpu_percent=excluded.alert_cpu_percent, alert_mem_percent=excluded.alert_mem_percent,
			alert_disk_percent=excluded.alert_disk_percent, alert_load_per_cpu=excluded.alert_load_per_cpu,
			alert_sustain_sec=excluded.alert_sustain_sec, alerts_enabled=excluded.alerts_enabled,
			client_version=excluded.client_version, last_sample_at=excluded.last_sample_at,
			last_poll_at=excluded.last_poll_at, last_poll_error=excluded.last_poll_error,
			last_boot_id=excluded.last_boot_id, updated_at=datetime('now')
	`, m.ID, m.Name, m.Host, m.Port, m.Username, m.EncSecret, m.SSHHostKey, m.FileServerID, enabled,
		m.PollIntervalSec, m.OfflineAfterSec, m.AlertCPUPercent, m.AlertMemPercent, m.AlertDiskPercent,
		m.AlertLoadPerCPU, m.AlertSustainSec, alerts, m.ClientVersion,
		m.LastSampleAt, m.LastPollAt, m.LastPollError, m.LastBootID, m.ID)
	return err
}

func (s *Store) DeleteMonitoredServer(id string) error {
	_, err := s.DB.Exec(`DELETE FROM monitored_servers WHERE id=?`, id)
	return err
}

func (s *Store) UpdateMonitoredServerPoll(id string, pollAt time.Time, pollErr string, clientVersion, bootID string, lastSampleAt *time.Time) error {
	var sample sql.NullString
	if lastSampleAt != nil && !lastSampleAt.IsZero() {
		sample = sql.NullString{String: lastSampleAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	_, err := s.DB.Exec(`
		UPDATE monitored_servers SET
			last_poll_at=?, last_poll_error=?,
			client_version=CASE WHEN ?!='' THEN ? ELSE client_version END,
			last_boot_id=CASE WHEN ?!='' THEN ? ELSE last_boot_id END,
			last_sample_at=CASE WHEN ?!='' THEN ? ELSE last_sample_at END,
			updated_at=datetime('now')
		WHERE id=?`,
		pollAt.UTC().Format(time.RFC3339), pollErr,
		clientVersion, clientVersion,
		bootID, bootID,
		sample.String, sample.String,
		id)
	return err
}

func (s *Store) PinMonitoredHostKey(id, fingerprint string) error {
	_, err := s.DB.Exec(`UPDATE monitored_servers SET ssh_host_key=?, updated_at=datetime('now') WHERE id=?`, fingerprint, id)
	return err
}

// InsertMonitorSample inserts a sample and its filesystems. Duplicate (server_id, sampled_at) is ignored.
// Network rates are derived from the previous sample's cumulative counters when safe.
func (s *Store) InsertMonitorSample(serverID string, sample metrics.Sample) (inserted bool, err error) {
	sampledAt := sample.SampledAt.UTC().Format(time.RFC3339Nano)
	tx, err := s.DB.Begin()
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	rxBps, txBps := deriveNetRates(tx, serverID, sample)

	res, err := tx.Exec(`
		INSERT OR IGNORE INTO monitor_samples(
			server_id, sampled_at, boot_id, uptime_sec, cpu_percent,
			mem_total_bytes, mem_used_bytes, mem_avail_bytes, swap_total_bytes, swap_used_bytes,
			load1, load5, load15, num_cpu, client_version,
			net_iface, net_rx_bytes, net_tx_bytes, net_rx_bps, net_tx_bps
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		serverID, sampledAt, sample.BootID, sample.UptimeSec, sample.CPUPercent,
		sample.MemTotalBytes, sample.MemUsedBytes, sample.MemAvailBytes, sample.SwapTotalBytes, sample.SwapUsedBytes,
		sample.Load1, sample.Load5, sample.Load15, sample.NumCPU, sample.ClientVersion,
		sample.NetIface, sample.NetRxBytes, sample.NetTxBytes, rxBps, txBps,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return false, tx.Commit()
	}
	var sampleID int64
	if err := tx.QueryRow(`SELECT id FROM monitor_samples WHERE server_id=? AND sampled_at=?`, serverID, sampledAt).Scan(&sampleID); err != nil {
		return false, err
	}
	for _, fs := range sample.Filesystems {
		if _, err := tx.Exec(`
			INSERT INTO monitor_filesystems(sample_id, server_id, sampled_at, mount, device, fs_type, total_bytes, used_bytes, free_bytes)
			VALUES(?,?,?,?,?,?,?,?,?)`,
			sampleID, serverID, sampledAt, fs.Mount, fs.Device, fs.FSType, fs.TotalBytes, fs.UsedBytes, fs.FreeBytes,
		); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

// deriveNetRates returns bytes/sec from consecutive cumulative counters, or nil when unsafe.
func deriveNetRates(tx *sql.Tx, serverID string, sample metrics.Sample) (rxBps, txBps any) {
	if sample.NetIface == "" {
		return nil, nil
	}
	var prevAt, prevIface string
	var prevRx, prevTx int64
	err := tx.QueryRow(`
		SELECT sampled_at, net_iface, net_rx_bytes, net_tx_bytes
		FROM monitor_samples
		WHERE server_id=? AND sampled_at < ?
		ORDER BY sampled_at DESC LIMIT 1`,
		serverID, sample.SampledAt.UTC().Format(time.RFC3339Nano),
	).Scan(&prevAt, &prevIface, &prevRx, &prevTx)
	if err != nil {
		return nil, nil
	}
	if prevIface == "" || prevIface != sample.NetIface {
		return nil, nil
	}
	prevTime, err := ParseStoreTime(prevAt)
	if err != nil {
		return nil, nil
	}
	dt := sample.SampledAt.UTC().Sub(prevTime.UTC()).Seconds()
	if dt < 1 || dt > 3600 {
		return nil, nil
	}
	if int64(sample.NetRxBytes) < prevRx || int64(sample.NetTxBytes) < prevTx {
		// Counter reset (reboot / interface recreate).
		return nil, nil
	}
	rx := float64(int64(sample.NetRxBytes)-prevRx) / dt
	txRate := float64(int64(sample.NetTxBytes)-prevTx) / dt
	return rx, txRate
}

func scanMonitorSample(row scannable) (MonitorSampleRow, error) {
	var r MonitorSampleRow
	var rxBps, txBps sql.NullFloat64
	err := row.Scan(
		&r.ID, &r.ServerID, &r.SampledAt, &r.BootID, &r.UptimeSec, &r.CPUPercent,
		&r.MemTotalBytes, &r.MemUsedBytes, &r.MemAvailBytes, &r.SwapTotalBytes, &r.SwapUsedBytes,
		&r.Load1, &r.Load5, &r.Load15, &r.NumCPU, &r.ClientVersion,
		&r.NetIface, &r.NetRxBytes, &r.NetTxBytes, &rxBps, &txBps,
	)
	if err != nil {
		return r, err
	}
	if rxBps.Valid {
		v := rxBps.Float64
		r.NetRxBps = &v
	}
	if txBps.Valid {
		v := txBps.Float64
		r.NetTxBps = &v
	}
	return r, nil
}

const monitorSampleCols = `id, server_id, sampled_at, boot_id, uptime_sec, cpu_percent,
		       mem_total_bytes, mem_used_bytes, mem_avail_bytes, swap_total_bytes, swap_used_bytes,
		       load1, load5, load15, num_cpu, client_version,
		       net_iface, net_rx_bytes, net_tx_bytes, net_rx_bps, net_tx_bps`

func (s *Store) LatestMonitorSample(serverID string) (*MonitorSampleRow, []MonitorFSRow, error) {
	row := s.DB.QueryRow(`
		SELECT `+monitorSampleCols+`
		FROM monitor_samples WHERE server_id=? ORDER BY sampled_at DESC LIMIT 1`, serverID)
	sample, err := scanMonitorSample(row)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	fs, err := s.listFSForSample(sample.ID)
	return &sample, fs, err
}

func (s *Store) listFSForSample(sampleID int64) ([]MonitorFSRow, error) {
	rows, err := s.DB.Query(`
		SELECT mount, device, fs_type, total_bytes, used_bytes, free_bytes
		FROM monitor_filesystems WHERE sample_id=? ORDER BY mount`, sampleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MonitorFSRow
	for rows.Next() {
		var f MonitorFSRow
		if err := rows.Scan(&f.Mount, &f.Device, &f.FSType, &f.TotalBytes, &f.UsedBytes, &f.FreeBytes); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) ListMonitorSamples(serverID string, since, until time.Time) ([]MonitorSampleRow, error) {
	rows, err := s.DB.Query(`
		SELECT `+monitorSampleCols+`
		FROM monitor_samples
		WHERE server_id=? AND sampled_at >= ? AND sampled_at <= ?
		ORDER BY sampled_at ASC`,
		serverID, since.UTC().Format(time.RFC3339Nano), until.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MonitorSampleRow
	for rows.Next() {
		row, err := scanMonitorSample(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) ListMonitorHourly(serverID string, since, until time.Time) ([]MonitorHourlyRow, error) {
	rows, err := s.DB.Query(`
		SELECT hour_at, samples, avg_cpu_percent, max_cpu_percent, avg_mem_percent, max_mem_percent,
		       avg_load1, max_load1, max_disk_percent,
		       avg_net_rx_bps, max_net_rx_bps, avg_net_tx_bps, max_net_tx_bps
		FROM monitor_hourly
		WHERE server_id=? AND hour_at >= ? AND hour_at <= ?
		ORDER BY hour_at ASC`,
		serverID, since.UTC().Format(time.RFC3339), until.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MonitorHourlyRow
	for rows.Next() {
		var r MonitorHourlyRow
		if err := rows.Scan(&r.HourAt, &r.Samples, &r.AvgCPUPercent, &r.MaxCPUPercent, &r.AvgMemPercent, &r.MaxMemPercent,
			&r.AvgLoad1, &r.MaxLoad1, &r.MaxDiskPercent,
			&r.AvgNetRxBps, &r.MaxNetRxBps, &r.AvgNetTxBps, &r.MaxNetTxBps); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) RollupMonitorHour(serverID string, hour time.Time) error {
	hour = hour.UTC().Truncate(time.Hour)
	start := hour.Format(time.RFC3339Nano)
	end := hour.Add(time.Hour).Format(time.RFC3339Nano)
	var samples int
	var avgCPU, maxCPU, avgLoad, maxLoad float64
	var avgMem, maxMem float64
	err := s.DB.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(AVG(cpu_percent),0), COALESCE(MAX(cpu_percent),0),
		       COALESCE(AVG(load1),0), COALESCE(MAX(load1),0),
		       COALESCE(AVG(CASE WHEN mem_total_bytes>0 THEN 100.0*mem_used_bytes/mem_total_bytes ELSE 0 END),0),
		       COALESCE(MAX(CASE WHEN mem_total_bytes>0 THEN 100.0*mem_used_bytes/mem_total_bytes ELSE 0 END),0)
		FROM monitor_samples
		WHERE server_id=? AND sampled_at >= ? AND sampled_at < ?`,
		serverID, start, end).
		Scan(&samples, &avgCPU, &maxCPU, &avgLoad, &maxLoad, &avgMem, &maxMem)
	if err != nil {
		return err
	}
	if samples == 0 {
		return nil
	}
	var maxDisk float64
	_ = s.DB.QueryRow(`
		SELECT COALESCE(MAX(CASE WHEN total_bytes>0 THEN 100.0*used_bytes/total_bytes ELSE 0 END),0)
		FROM monitor_filesystems
		WHERE server_id=? AND sampled_at >= ? AND sampled_at < ?`,
		serverID, start, end).Scan(&maxDisk)

	var avgRx, maxRx, avgTx, maxTx float64
	_ = s.DB.QueryRow(`
		SELECT COALESCE(AVG(net_rx_bps),0), COALESCE(MAX(net_rx_bps),0),
		       COALESCE(AVG(net_tx_bps),0), COALESCE(MAX(net_tx_bps),0)
		FROM monitor_samples
		WHERE server_id=? AND sampled_at >= ? AND sampled_at < ? AND net_rx_bps IS NOT NULL`,
		serverID, start, end).Scan(&avgRx, &maxRx, &avgTx, &maxTx)

	_, err = s.DB.Exec(`
		INSERT INTO monitor_hourly(server_id, hour_at, samples, avg_cpu_percent, max_cpu_percent,
			avg_mem_percent, max_mem_percent, avg_load1, max_load1, max_disk_percent,
			avg_net_rx_bps, max_net_rx_bps, avg_net_tx_bps, max_net_tx_bps)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(server_id, hour_at) DO UPDATE SET
			samples=excluded.samples, avg_cpu_percent=excluded.avg_cpu_percent, max_cpu_percent=excluded.max_cpu_percent,
			avg_mem_percent=excluded.avg_mem_percent, max_mem_percent=excluded.max_mem_percent,
			avg_load1=excluded.avg_load1, max_load1=excluded.max_load1, max_disk_percent=excluded.max_disk_percent,
			avg_net_rx_bps=excluded.avg_net_rx_bps, max_net_rx_bps=excluded.max_net_rx_bps,
			avg_net_tx_bps=excluded.avg_net_tx_bps, max_net_tx_bps=excluded.max_net_tx_bps`,
		serverID, hour.Format(time.RFC3339), samples, avgCPU, maxCPU, avgMem, maxMem, avgLoad, maxLoad, maxDisk,
		avgRx, maxRx, avgTx, maxTx)
	return err
}

func (s *Store) PruneMonitorData(rawKeepDays, hourlyKeepDays int) error {
	if rawKeepDays < 1 {
		rawKeepDays = 30
	}
	if hourlyKeepDays < 1 {
		hourlyKeepDays = 365
	}
	rawCut := time.Now().UTC().AddDate(0, 0, -rawKeepDays).Format(time.RFC3339Nano)
	hourCut := time.Now().UTC().AddDate(0, 0, -hourlyKeepDays).Format(time.RFC3339)
	if _, err := s.DB.Exec(`DELETE FROM monitor_filesystems WHERE sampled_at < ?`, rawCut); err != nil {
		return err
	}
	if _, err := s.DB.Exec(`DELETE FROM monitor_samples WHERE sampled_at < ?`, rawCut); err != nil {
		return err
	}
	_, err := s.DB.Exec(`DELETE FROM monitor_hourly WHERE hour_at < ?`, hourCut)
	return err
}

func (s *Store) GetMonitorAlertState(serverID, key string) (*MonitorAlertState, error) {
	var a MonitorAlertState
	var active int
	err := s.DB.QueryRow(`
		SELECT server_id, alert_key, active, since_at, last_sent_at, last_value
		FROM monitor_alert_state WHERE server_id=? AND alert_key=?`, serverID, key).
		Scan(&a.ServerID, &a.AlertKey, &active, &a.SinceAt, &a.LastSentAt, &a.LastValue)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.Active = active != 0
	return &a, nil
}

func (s *Store) UpsertMonitorAlertState(a MonitorAlertState) error {
	active := 0
	if a.Active {
		active = 1
	}
	_, err := s.DB.Exec(`
		INSERT INTO monitor_alert_state(server_id, alert_key, active, since_at, last_sent_at, last_value)
		VALUES(?,?,?,?,?,?)
		ON CONFLICT(server_id, alert_key) DO UPDATE SET
			active=excluded.active, since_at=excluded.since_at, last_sent_at=excluded.last_sent_at, last_value=excluded.last_value`,
		a.ServerID, a.AlertKey, active, a.SinceAt, a.LastSentAt, a.LastValue)
	return err
}

func (s *Store) ListActiveMonitorAlerts(serverID string) ([]MonitorAlertState, error) {
	rows, err := s.DB.Query(`
		SELECT server_id, alert_key, active, since_at, last_sent_at, last_value
		FROM monitor_alert_state WHERE server_id=? AND active=1`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MonitorAlertState
	for rows.Next() {
		var a MonitorAlertState
		var active int
		if err := rows.Scan(&a.ServerID, &a.AlertKey, &active, &a.SinceAt, &a.LastSentAt, &a.LastValue); err != nil {
			return nil, err
		}
		a.Active = active != 0
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) CountMonitoredServers() (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM monitored_servers`).Scan(&n)
	return n, err
}

func (s *Store) CountOnlineMonitoredServers(now time.Time) (int, error) {
	servers, err := s.ListMonitoredServers()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, m := range servers {
		if !m.Enabled {
			continue
		}
		if !m.LastSampleAt.Valid {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, m.LastSampleAt.String)
		if err != nil {
			t, err = time.Parse(time.RFC3339, m.LastSampleAt.String)
		}
		if err != nil {
			continue
		}
		grace := time.Duration(m.OfflineAfterSec) * time.Second
		if grace < time.Minute {
			grace = 3 * time.Minute
		}
		if now.Sub(t) <= grace {
			n++
		}
	}
	return n, nil
}

func FormatNullTime(t *time.Time) sql.NullString {
	if t == nil || t.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(time.RFC3339Nano), Valid: true}
}

func ParseStoreTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized time %q", s)
}
