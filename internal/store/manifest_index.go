package store

import (
	"strings"

	"github.com/boomerang-backup/boomerang/internal/backup"
)

type ManifestFile struct {
	Path  string
	Size  int64
	IsDir bool
	Mtime string
}

func (s *Store) ReplaceManifestIndex(versionID string, entries []backup.ManifestEntry) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM manifest_files WHERE version_id=?`, versionID); err != nil {
		_ = tx.Rollback()
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO manifest_files(version_id, path, size, is_dir, mtime) VALUES(?,?,?,?,?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, e := range entries {
		isDir := 0
		if e.IsDir {
			isDir = 1
		}
		if _, err := stmt.Exec(versionID, e.Path, e.Size, isDir, e.Mtime); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) HasManifestIndex(versionID string) (bool, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM manifest_files WHERE version_id=? LIMIT 1`, versionID).Scan(&n)
	return n > 0, err
}

func (s *Store) ListManifestFiles(versionID string) ([]ManifestFile, error) {
	rows, err := s.DB.Query(`SELECT path, size, is_dir, mtime FROM manifest_files WHERE version_id=?`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ManifestFile
	for rows.Next() {
		var f ManifestFile
		var isDir int
		if err := rows.Scan(&f.Path, &f.Size, &isDir, &f.Mtime); err != nil {
			return nil, err
		}
		f.IsDir = isDir != 0
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) SearchManifestChain(chain []string, query string, limit int) ([]ManifestFile, error) {
	if limit <= 0 {
		limit = 500
	}
	q := strings.ToLower(strings.TrimSpace(query))
	merged := map[string]ManifestFile{}
	for _, vid := range chain {
		rows, err := s.DB.Query(`SELECT path, size, is_dir, mtime FROM manifest_files WHERE version_id=?`, vid)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var f ManifestFile
			var isDir int
			if err := rows.Scan(&f.Path, &f.Size, &isDir, &f.Mtime); err != nil {
				rows.Close()
				return nil, err
			}
			f.IsDir = isDir != 0
			if f.IsDir {
				continue
			}
			p := strings.Trim(f.Path, "/")
			if p == "" || !strings.Contains(strings.ToLower(p), q) {
				continue
			}
			merged[p] = f
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	out := make([]ManifestFile, 0, len(merged))
	for _, f := range merged {
		out = append(out, f)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *Store) CountManifestChain(chain []string) (int, error) {
	merged := map[string]bool{}
	for _, vid := range chain {
		rows, err := s.DB.Query(`SELECT path FROM manifest_files WHERE version_id=? AND is_dir=0`, vid)
		if err != nil {
			return 0, err
		}
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err != nil {
				rows.Close()
				return 0, err
			}
			merged[strings.Trim(p, "/")] = true
		}
		rows.Close()
	}
	return len(merged), nil
}

// BrowseManifestChain returns immediate children under prefix from indexed chain versions.
func (s *Store) BrowseManifestChain(chain []string, prefix string) ([]ManifestFile, error) {
	prefix = strings.Trim(prefix, "/")
	merged := map[string]ManifestFile{}
	for _, vid := range chain {
		rows, err := s.DB.Query(`SELECT path, size, is_dir, mtime FROM manifest_files WHERE version_id=?`, vid)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var f ManifestFile
			var isDir int
			if err := rows.Scan(&f.Path, &f.Size, &isDir, &f.Mtime); err != nil {
				rows.Close()
				return nil, err
			}
			f.IsDir = isDir != 0
			p := strings.Trim(f.Path, "/")
			if p == "" || p == "." {
				continue
			}
			if prefix != "" {
				if p == prefix {
					continue
				}
				if !strings.HasPrefix(p, prefix+"/") {
					continue
				}
			}
			merged[p] = f
		}
		rows.Close()
	}
	children := map[string]ManifestFile{}
	for p, f := range merged {
		var rest string
		if prefix == "" {
			rest = p
		} else {
			rest = strings.TrimPrefix(p, prefix+"/")
		}
		name, more, _ := strings.Cut(rest, "/")
		if name == "" {
			continue
		}
		full := name
		if prefix != "" {
			full = prefix + "/" + name
		}
		if more != "" {
			if cur, ok := children[name]; ok {
				cur.IsDir = true
				children[name] = cur
			} else {
				children[name] = ManifestFile{Path: full, IsDir: true}
			}
			continue
		}
		children[name] = ManifestFile{Path: full, Size: f.Size, IsDir: f.IsDir, Mtime: f.Mtime}
	}
	out := make([]ManifestFile, 0, len(children))
	for _, f := range children {
		out = append(out, f)
	}
	return out, nil
}
