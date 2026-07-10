package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// PurgeTarget removes all backup versions (and on-disk files), jobs, and job logs
// for a file server or database target.
func (s *Store) PurgeTarget(targetType, targetID string) error {
	versions, err := s.ListVersions(targetType, targetID)
	if err != nil {
		return err
	}
	for _, v := range versions {
		if v.PathOnDisk != "" {
			_ = os.RemoveAll(v.PathOnDisk)
		}
	}
	if _, err := s.DB.Exec(`DELETE FROM backup_versions WHERE target_type=? AND target_id=?`, targetType, targetID); err != nil {
		return fmt.Errorf("delete versions: %w", err)
	}
	// job_logs cascade via FK when jobs are deleted; delete explicitly for safety
	if _, err := s.DB.Exec(`
		DELETE FROM job_logs WHERE job_id IN (
			SELECT id FROM jobs WHERE target_type=? AND target_id=?
		)`, targetType, targetID); err != nil {
		return fmt.Errorf("delete job logs: %w", err)
	}
	if _, err := s.DB.Exec(`DELETE FROM jobs WHERE target_type=? AND target_id=?`, targetType, targetID); err != nil {
		return fmt.Errorf("delete jobs: %w", err)
	}
	return nil
}

// CleanupOrphans removes DB rows and backup directories whose targets no longer exist.
func (s *Store) CleanupOrphans(dataDir string) (int, error) {
	n := 0

	// Orphan file versions / jobs
	rows, err := s.DB.Query(`
		SELECT DISTINCT target_id FROM backup_versions WHERE target_type='file'
		UNION
		SELECT DISTINCT target_id FROM jobs WHERE target_type='file'`)
	if err != nil {
		return 0, err
	}
	var fileIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return n, err
		}
		fileIDs = append(fileIDs, id)
	}
	_ = rows.Close()
	for _, id := range fileIDs {
		f, err := s.GetFileServer(id)
		if err != nil {
			return n, err
		}
		if f == nil {
			if err := s.PurgeTarget("file", id); err != nil {
				return n, err
			}
			_ = os.RemoveAll(filepath.Join(dataDir, "backups", "files", id))
			n++
		}
	}

	rows, err = s.DB.Query(`
		SELECT DISTINCT target_id FROM backup_versions WHERE target_type='db'
		UNION
		SELECT DISTINCT target_id FROM jobs WHERE target_type='db'`)
	if err != nil {
		return n, err
	}
	var dbIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return n, err
		}
		dbIDs = append(dbIDs, id)
	}
	_ = rows.Close()
	for _, id := range dbIDs {
		d, err := s.GetDatabase(id)
		if err != nil {
			return n, err
		}
		if d == nil {
			if err := s.PurgeTarget("db", id); err != nil {
				return n, err
			}
			_ = os.RemoveAll(filepath.Join(dataDir, "backups", "db", id))
			n++
		}
	}

	// Orphan directories on disk with no matching target row
	n += purgeOrphanDirs(filepath.Join(dataDir, "backups", "files"), func(id string) bool {
		f, _ := s.GetFileServer(id)
		return f == nil
	})
	n += purgeOrphanDirs(filepath.Join(dataDir, "backups", "db"), func(id string) bool {
		d, _ := s.GetDatabase(id)
		return d == nil
	})

	return n, nil
}

func purgeOrphanDirs(root string, isOrphan func(id string) bool) int {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		if isOrphan(id) {
			_ = os.RemoveAll(filepath.Join(root, id))
			n++
		}
	}
	return n
}
