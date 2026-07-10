package store

import "time"

type StorageForecast struct {
	CurrentBytes   int64   `json:"currentBytes"`
	DailyBytes     int64   `json:"dailyBytes"`
	Projected30Day int64   `json:"projected30Day"`
	SampleDays     int     `json:"sampleDays"`
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
	out.Projected30Day = cur + out.DailyBytes*30
	return out, nil
}
