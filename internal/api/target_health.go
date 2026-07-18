package api

import (
	"net/http"
	"time"

	"github.com/boomerang-backup/boomerang/internal/targethealth"
	"github.com/boomerang-backup/boomerang/internal/tzutil"
)

type targetHealthRow struct {
	ID                 string `json:"id"`
	TargetType         string `json:"targetType"`
	Name               string `json:"name"`
	Enabled            bool   `json:"enabled"`
	Scheduled          bool   `json:"scheduled"`
	LastSuccessAt      string `json:"lastSuccessAt,omitempty"`
	LastSuccessBytes   int64  `json:"lastSuccessBytes,omitempty"`
	LastCheckAt        string `json:"lastCheckAt,omitempty"`
	LastWasSkip        bool   `json:"lastWasSkip,omitempty"`
	VersionCount       int    `json:"versionCount"`
	LastJobStatus      string `json:"lastJobStatus,omitempty"`
	LastJobError       string `json:"lastJobError,omitempty"`
	NextRunAt          string `json:"nextRunAt,omitempty"`
	StorageBytes       int64  `json:"storageBytes"`
	Health             string `json:"health"`
	HealthDetail       string `json:"healthDetail,omitempty"`
	MonitoredServerID  string `json:"monitoredServerId,omitempty"`
	MonitoredServerName string `json:"monitoredServerName,omitempty"`
}

func (s *Server) handleTargetHealth(w http.ResponseWriter, _ *http.Request) {
	var out []targetHealthRow
	loc := tzutil.Load(s.store)
	now := time.Now()
	monitors, _ := s.store.MonitoredByFileServerID()

	files, _ := s.store.ListFileServers()
	for _, f := range files {
		r := targethealth.Evaluate(s.store, "file", f.ID, f.Name, f.Enabled, f.ScheduleCron, f.ScheduleStart, now, loc)
		row := healthRowFrom(r)
		if bytes, err := s.store.SumTargetBackupBytes("file", f.ID); err == nil {
			row.StorageBytes = bytes
		}
		if m, ok := monitors[f.ID]; ok {
			row.MonitoredServerID = m.ID
			row.MonitoredServerName = m.Name
		}
		out = append(out, row)
	}
	dbs, _ := s.store.ListDatabases()
	for _, d := range dbs {
		r := targethealth.Evaluate(s.store, "db", d.ID, d.Name, d.Enabled, d.ScheduleCron, d.ScheduleStart, now, loc)
		row := healthRowFrom(r)
		if bytes, err := s.store.SumTargetBackupBytes("db", d.ID); err == nil {
			row.StorageBytes = bytes
		}
		out = append(out, row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"targets": out})
}

func healthRowFrom(r targethealth.Result) targetHealthRow {
	return targetHealthRow{
		ID: r.ID, TargetType: r.TargetType, Name: r.Name, Enabled: r.Enabled, Scheduled: r.Scheduled,
		LastSuccessAt: r.LastSuccessAt, LastSuccessBytes: r.LastSuccessBytes,
		LastCheckAt: r.LastCheckAt, LastWasSkip: r.LastWasSkip,
		VersionCount: r.VersionCount,
		LastJobStatus: r.LastJobStatus, LastJobError: r.LastJobError, NextRunAt: r.NextRunAt,
		Health: r.Health, HealthDetail: r.HealthDetail,
	}
}

func scheduleActive(start string) bool {
	return targethealth.ScheduleActive(start, time.Now().UTC())
}

func parseHealthTime(s string) (time.Time, bool) {
	return targethealth.ParseTime(s)
}
