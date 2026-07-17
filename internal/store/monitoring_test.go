package store

import (
	"testing"
	"time"

	"github.com/boomerang-backup/boomerang/internal/metrics"
)

func TestMonitorSampleInsertIdempotent(t *testing.T) {
	st, err := Open(t.TempDir() + "/app.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	m := &MonitoredServer{
		ID: "s1", Name: "box", Host: "10.0.0.1", Port: 22, Username: "boomerang-monitor",
		Enabled: true, AlertsEnabled: true, PollIntervalSec: 60, OfflineAfterSec: 180,
		AlertCPUPercent: 90, AlertMemPercent: 90, AlertDiskPercent: 90, AlertLoadPerCPU: 2, AlertSustainSec: 300,
	}
	if err := st.UpsertMonitoredServer(m); err != nil {
		t.Fatal(err)
	}
	sample := metrics.Sample{
		SchemaVersion: 1,
		SampledAt:     time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC),
		CPUPercent:    12,
		MemTotalBytes: 1000,
		MemUsedBytes:  400,
		Load1:         0.5,
		NumCPU:        2,
		Filesystems: []metrics.Filesystem{
			{Mount: "/", TotalBytes: 100, UsedBytes: 40, FreeBytes: 60},
		},
	}
	ok, err := st.InsertMonitorSample("s1", sample)
	if err != nil || !ok {
		t.Fatalf("first insert: ok=%v err=%v", ok, err)
	}
	ok, err = st.InsertMonitorSample("s1", sample)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("duplicate should not insert")
	}
	if err := st.RollupMonitorHour("s1", sample.SampledAt); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListMonitorHourly("s1", sample.SampledAt.Add(-time.Hour), sample.SampledAt.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Samples != 1 {
		t.Fatalf("hourly = %+v", rows)
	}
}

func TestMonitorNetworkRates(t *testing.T) {
	st, err := Open(t.TempDir() + "/net.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	m := &MonitoredServer{
		ID: "s1", Name: "box", Host: "10.0.0.1", Port: 22, Username: "boomerang-monitor",
		Enabled: true, AlertsEnabled: true, PollIntervalSec: 60, OfflineAfterSec: 180,
		AlertCPUPercent: 90, AlertMemPercent: 90, AlertDiskPercent: 90, AlertLoadPerCPU: 2, AlertSustainSec: 300,
	}
	if err := st.UpsertMonitoredServer(m); err != nil {
		t.Fatal(err)
	}

	t0 := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	first := metrics.Sample{
		SampledAt: t0, NetIface: "eth0", NetRxBytes: 1000, NetTxBytes: 2000,
	}
	ok, err := st.InsertMonitorSample("s1", first)
	if err != nil || !ok {
		t.Fatalf("first: %v %v", ok, err)
	}
	latest, _, err := st.LatestMonitorSample("s1")
	if err != nil || latest == nil {
		t.Fatal(err)
	}
	if latest.NetRxBps != nil || latest.NetTxBps != nil {
		t.Fatalf("first sample should have nil rates, got %+v %+v", latest.NetRxBps, latest.NetTxBps)
	}

	second := metrics.Sample{
		SampledAt: t0.Add(time.Minute), NetIface: "eth0",
		NetRxBytes: 1000 + 6000, NetTxBytes: 2000 + 12000, // 100 B/s RX, 200 B/s TX over 60s
	}
	ok, err = st.InsertMonitorSample("s1", second)
	if err != nil || !ok {
		t.Fatalf("second: %v %v", ok, err)
	}
	latest, _, err = st.LatestMonitorSample("s1")
	if err != nil || latest == nil || latest.NetRxBps == nil || latest.NetTxBps == nil {
		t.Fatalf("expected rates: %+v err=%v", latest, err)
	}
	if *latest.NetRxBps != 100 || *latest.NetTxBps != 200 {
		t.Fatalf("rates = %.1f/%.1f, want 100/200", *latest.NetRxBps, *latest.NetTxBps)
	}

	if err := st.RollupMonitorHour("s1", t0); err != nil {
		t.Fatal(err)
	}
	hours, err := st.ListMonitorHourly("s1", t0.Add(-time.Hour), t0.Add(2*time.Hour))
	if err != nil || len(hours) != 1 {
		t.Fatalf("hourly: %v %+v", err, hours)
	}
	if hours[0].AvgNetRxBps != 100 || hours[0].MaxNetTxBps != 200 {
		t.Fatalf("hourly net = %+v", hours[0])
	}

	// Counter reset should clear rates.
	reset := metrics.Sample{
		SampledAt: t0.Add(2 * time.Minute), NetIface: "eth0",
		NetRxBytes: 10, NetTxBytes: 10,
	}
	if _, err := st.InsertMonitorSample("s1", reset); err != nil {
		t.Fatal(err)
	}
	latest, _, _ = st.LatestMonitorSample("s1")
	if latest.NetRxBps != nil || latest.NetTxBps != nil {
		t.Fatalf("reset should nil rates, got %+v %+v", latest.NetRxBps, latest.NetTxBps)
	}

	// Interface change should clear rates.
	a := metrics.Sample{SampledAt: t0.Add(3 * time.Minute), NetIface: "eth0", NetRxBytes: 100, NetTxBytes: 100}
	b := metrics.Sample{SampledAt: t0.Add(4 * time.Minute), NetIface: "wlan0", NetRxBytes: 10000, NetTxBytes: 10000}
	_, _ = st.InsertMonitorSample("s1", a)
	_, _ = st.InsertMonitorSample("s1", b)
	latest, _, _ = st.LatestMonitorSample("s1")
	if latest.NetRxBps != nil {
		t.Fatalf("iface change should nil rates")
	}
}
