package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type Store struct {
	DB *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}

func (s *Store) migrate() error {
	if _, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`); err != nil {
		return err
	}

	// Existing installs already applied 001 before tracking existed.
	var n int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='file_servers'`).Scan(&n)
	if n > 0 {
		_, _ = s.DB.Exec(`INSERT OR IGNORE INTO schema_migrations(name) VALUES('001_init.sql')`)
	}

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		var applied string
		err := s.DB.QueryRow(`SELECT name FROM schema_migrations WHERE name = ?`, name).Scan(&applied)
		if err == nil {
			continue
		}
		if err != sql.ErrNoRows {
			return err
		}
		sqlBytes, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := s.DB.Exec(string(sqlBytes)); err != nil {
			return fmt.Errorf("migrate %s: %w", name, err)
		}
		if _, err := s.DB.Exec(`INSERT INTO schema_migrations(name) VALUES(?)`, name); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetMeta(key string) (string, bool, error) {
	var v string
	err := s.DB.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (s *Store) SetMeta(key, value string) error {
	_, err := s.DB.Exec(`
		INSERT INTO meta(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

func (s *Store) IsSetup() (bool, error) {
	_, ok, err := s.GetMeta("admin_password_hash")
	return ok, err
}

func (s *Store) Audit(action, detail string) error {
	_, err := s.DB.Exec(`INSERT INTO audit_log(action, detail) VALUES(?, ?)`, action, detail)
	return err
}

func (s *Store) CountFileServers() (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM file_servers`).Scan(&n)
	return n, err
}

func (s *Store) CountDatabases() (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM databases`).Scan(&n)
	return n, err
}
