package store

type VersionSizePoint struct {
	Bytes     int64  `json:"bytes"`
	CreatedAt string `json:"createdAt"`
}

func (s *Store) RecentVersionSizes(targetType, targetID string, limit int) ([]VersionSizePoint, error) {
	if limit <= 0 {
		limit = 7
	}
	if limit > 30 {
		limit = 30
	}
	rows, err := s.DB.Query(`
		SELECT bytes, created_at FROM backup_versions
		WHERE target_type=? AND target_id=? AND status='succeeded'
		ORDER BY created_at DESC LIMIT ?`, targetType, targetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VersionSizePoint
	for rows.Next() {
		var p VersionSizePoint
		if err := rows.Scan(&p.Bytes, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	// oldest first for sparklines
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}
