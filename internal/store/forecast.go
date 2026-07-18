package store

import (
	"fmt"
	"math"
	"time"
)

type StorageForecast struct {
	CurrentBytes     int64            `json:"currentBytes"`
	DailyBytes       int64            `json:"dailyBytes"`
	NetDailyBytes    int64            `json:"netDailyBytes"`
	SteadyStateBytes int64            `json:"steadyStateBytes"`
	Projected30Day   int64            `json:"projected30Day"`
	SampleDays       int              `json:"sampleDays"`
	GrowthByTier     map[string]int64 `json:"growthByTier,omitempty"`
	DominantTier    string           `json:"dominantTier,omitempty"`
	DominantSharePct int              `json:"dominantSharePct,omitempty"`
	RateBytesPerDay  int64            `json:"rateBytesPerDay"`
	HitBytes         int64            `json:"hitBytes,omitempty"`
	HitAt            string           `json:"hitAt,omitempty"`
	Assumptions      string           `json:"assumptions"`
}

func (s *Store) StorageForecast(sampleDays int) (StorageForecast, error) {
	out := StorageForecast{SampleDays: sampleDays}
	if sampleDays <= 0 {
		sampleDays = 7
		out.SampleDays = sampleDays
	}
	out.Assumptions = fmt.Sprintf(
		"Based on the last %d days of net storage change; capped by configured retention; ignores prune timing and size changes within existing slots.",
		sampleDays,
	)

	cur, err := s.SumBackupBytes()
	if err != nil {
		return out, err
	}
	out.CurrentBytes = cur

	cutTime := time.Now().UTC().AddDate(0, 0, -sampleDays)
	cut := cutTime.Format(time.RFC3339)
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
	rate := out.NetDailyBytes
	if rate == 0 && out.DailyBytes > 0 {
		rate = out.DailyBytes
	}
	out.RateBytesPerDay = rate

	projected := cur + rate*30
	if out.SteadyStateBytes > 0 && projected > out.SteadyStateBytes {
		projected = out.SteadyStateBytes
	}
	out.Projected30Day = projected

	out.GrowthByTier = s.growthByRetentionTier(cutTime)
	out.DominantTier, out.DominantSharePct = dominantTier(out.GrowthByTier)

	out.HitBytes = projected
	if rate > 0 && out.HitBytes > cur {
		days := int(math.Ceil(float64(out.HitBytes-cur) / float64(rate)))
		if days < 1 {
			days = 1
		}
		out.HitAt = time.Now().UTC().AddDate(0, 0, days).Format(time.RFC3339)
	}

	return out, nil
}

func dominantTier(by map[string]int64) (string, int) {
	if len(by) == 0 {
		return "", 0
	}
	var best string
	var bestN, total int64
	for tier, n := range by {
		if n <= 0 {
			continue
		}
		total += n
		if n > bestN {
			bestN = n
			best = tier
		}
	}
	if best == "" || total <= 0 {
		return "", 0
	}
	return best, int((bestN * 100) / total)
}

func (s *Store) growthByRetentionTier(since time.Time) map[string]int64 {
	out := map[string]int64{}
	files, _ := s.ListFileServers()
	for _, f := range files {
		versions, err := s.ListVersions("file", f.ID)
		if err != nil {
			continue
		}
		r := Retention{
			Hourly: f.RetainHourly, Daily: f.RetainDaily, Weekly: f.RetainWeekly,
			Monthly: f.RetainMonthly, Yearly: f.RetainYearly, Count: f.RetainCount,
		}
		addTierGrowth(out, versions, r, since)
	}
	dbs, _ := s.ListDatabases()
	for _, d := range dbs {
		versions, err := s.ListVersions("db", d.ID)
		if err != nil {
			continue
		}
		r := Retention{
			Hourly: d.RetainHourly, Daily: d.RetainDaily, Weekly: d.RetainWeekly,
			Monthly: d.RetainMonthly, Yearly: d.RetainYearly, Count: d.RetainCount,
		}
		addTierGrowth(out, versions, r, since)
	}
	return out
}

func addTierGrowth(out map[string]int64, versions []Version, r Retention, since time.Time) {
	gfs := r.Hourly > 0 || r.Daily > 0 || r.Weekly > 0 || r.Monthly > 0 || r.Yearly > 0
	if !gfs {
		// Legacy count retention — attribute recent growth as daily.
		for _, v := range versions {
			if v.Status != "succeeded" {
				continue
			}
			t, ok := parseVersionTime(v.CreatedAt)
			if !ok || t.Before(since) {
				continue
			}
			out["daily"] += v.Bytes
		}
		return
	}
	primary := primaryRetentionTiers(versions, r)
	for _, v := range versions {
		tier, ok := primary[v.ID]
		if !ok {
			continue
		}
		t, ok := parseVersionTime(v.CreatedAt)
		if !ok || t.Before(since) {
			continue
		}
		out[tier] += v.Bytes
	}
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
