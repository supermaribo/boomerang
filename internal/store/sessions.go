package store

import (
	"database/sql"
	"time"
)

func (s *Store) SaveSession(token string, epoch int, expires time.Time) error {
	_, err := s.DB.Exec(`INSERT INTO sessions(token, epoch, expires_at) VALUES(?,?,?)
		ON CONFLICT(token) DO UPDATE SET epoch=excluded.epoch, expires_at=excluded.expires_at`,
		token, epoch, expires.UTC().Format(time.RFC3339))
	return err
}

func (s *Store) GetSession(token string) (epoch int, expires time.Time, ok bool, err error) {
	var expStr string
	err = s.DB.QueryRow(`SELECT epoch, expires_at FROM sessions WHERE token=?`, token).Scan(&epoch, &expStr)
	if err == sql.ErrNoRows {
		return 0, time.Time{}, false, nil
	}
	if err != nil {
		return 0, time.Time{}, false, err
	}
	expires, parseErr := time.Parse(time.RFC3339, expStr)
	if parseErr != nil {
		if expires, parseErr = time.Parse("2006-01-02 15:04:05", expStr); parseErr != nil {
			return 0, time.Time{}, false, nil
		}
	}
	return epoch, expires.UTC(), true, nil
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE token=?`, token)
	return err
}

func (s *Store) PruneSessions() error {
	cut := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE expires_at < ?`, cut)
	return err
}

func (s *Store) DeleteAllSessions() error {
	_, err := s.DB.Exec(`DELETE FROM sessions`)
	return err
}
