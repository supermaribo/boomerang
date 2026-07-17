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
