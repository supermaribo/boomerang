package store

import "time"

type StorageForecast struct {
	CurrentBytes      int64 `json:"currentBytes"`
	DailyBytes        int64 `json:"dailyBytes"`
	NetDailyBytes     int64 `json:"netDailyBytes"`
	SteadyStateBytes  int64 `json:"steadyStateBytes"`
	Projected30Day    int64 `json:"projected30Day"`
	SampleDays        int   `json:"sampleDays"`
}

func (s *Store) StorageForecast(sampleDays int) (StorageForecast, error) {
	out := StorageForecast{SampleDays: sampleDays}
	if sampleDays <= 0 {
		sampleDays = 7
		out.SampleDays = sampleDays
	}
	cur, err := s.SumBackupBytes()
	if err != nil {
		return out, err
	}
	out.CurrentBytes = cur

	cut := time.Now().UTC().AddDate(0, 0, -sampleDays).Format(time.RFC3339)
	var added int64
	err = s.DB.QueryRow(`
		SELECT COALESCE(SUM(bytes), 0) FROM backup_versions
		WHERE status='succeeded' AND created_at >= ?`, cut).Scan(&added)
	if err != nil {
		return out, err
	}
	if added > 0 {
		out.DailyBytes = added / int64(sampleDays)
		if out.DailyBytes < 1 {
			out.DailyBytes = 1
		}
	}

	// Net growth: compare current total to total bytes of versions that existed at cutoff.
	var atCutoff int64
	_ = s.DB.QueryRow(`
		SELECT COALESCE(SUM(bytes), 0) FROM backup_versions
		WHERE status='succeeded' AND created_at < ?`, cut).Scan(&atCutoff)
	net := cur - atCutoff
	if net < 0 {
		net = 0
	}
	out.NetDailyBytes = net / int64(sampleDays)

	out.SteadyStateBytes = s.estimateSteadyStateBytes()
	projected := cur + out.NetDailyBytes*30
	if out.NetDailyBytes == 0 && out.DailyBytes > 0 {
		projected = cur + out.DailyBytes*30
	}
	if out.SteadyStateBytes > 0 && projected > out.SteadyStateBytes {
		projected = out.SteadyStateBytes
	}
	out.Projected30Day = projected
	return out, nil
}

func (s *Store) estimateSteadyStateBytes() int64 {
	var total int64
	files, _ := s.ListFileServers()
	for _, f := range files {
		slots := retentionSlots(f.RetainHourly, f.RetainDaily, f.RetainWeekly, f.RetainMonthly, f.RetainYearly, f.RetainCount)
		avg, _ := s.avgRecentBackupBytes("file", f.ID, 3)
		total += avg * int64(slots)
	}
	dbs, _ := s.ListDatabases()
	for _, d := range dbs {
		slots := retentionSlots(d.RetainHourly, d.RetainDaily, d.RetainWeekly, d.RetainMonthly, d.RetainYearly, d.RetainCount)
		avg, _ := s.avgRecentBackupBytes("db", d.ID, 3)
		total += avg * int64(slots)
	}
	return total
}

func retentionSlots(hourly, daily, weekly, monthly, yearly, count int) int {
	n := hourly + daily + weekly + monthly + yearly
	if n <= 0 && count > 0 {
		return count
	}
	if n <= 0 {
		return 7
	}
	return n
}

func (s *Store) avgRecentBackupBytes(targetType, targetID string, limit int) (int64, error) {
	points, err := s.RecentVersionSizes(targetType, targetID, limit)
	if err != nil || len(points) == 0 {
		return 0, err
	}
	var sum int64
	for _, p := range points {
		sum += p.Bytes
	}
	return sum / int64(len(points)), nil
}
