CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT PRIMARY KEY,
    epoch      INTEGER NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
