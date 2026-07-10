CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    action     TEXT NOT NULL,
    detail     TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS file_servers (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    protocol      TEXT NOT NULL DEFAULT 'sftp',
    host          TEXT NOT NULL,
    port          INTEGER NOT NULL DEFAULT 22,
    username      TEXT NOT NULL,
    remote_root   TEXT NOT NULL DEFAULT '/',
    auth_mode     TEXT NOT NULL DEFAULT 'password',
    enc_secret    BLOB,
    schedule_cron TEXT NOT NULL DEFAULT '0 2 * * *',
    retain_count  INTEGER NOT NULL DEFAULT 14,
    retain_days   INTEGER NOT NULL DEFAULT 0,
    enabled       INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS databases (
    id                 TEXT PRIMARY KEY,
    name               TEXT NOT NULL,
    mysql_host         TEXT NOT NULL DEFAULT '127.0.0.1',
    mysql_port         INTEGER NOT NULL DEFAULT 3306,
    mysql_db           TEXT NOT NULL,
    mysql_user         TEXT NOT NULL,
    enc_mysql_password BLOB,
    tunnel_mode        TEXT NOT NULL DEFAULT 'none',
    file_server_id     TEXT,
    ssh_host           TEXT,
    ssh_port           INTEGER NOT NULL DEFAULT 22,
    ssh_username       TEXT,
    auth_mode          TEXT NOT NULL DEFAULT 'password',
    enc_ssh_secret     BLOB,
    schedule_cron      TEXT NOT NULL DEFAULT '0 2 * * *',
    retain_count       INTEGER NOT NULL DEFAULT 14,
    retain_days        INTEGER NOT NULL DEFAULT 0,
    enabled            INTEGER NOT NULL DEFAULT 1,
    created_at         TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (file_server_id) REFERENCES file_servers(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS jobs (
    id           TEXT PRIMARY KEY,
    target_type  TEXT NOT NULL,
    target_id    TEXT NOT NULL,
    kind         TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'queued',
    error        TEXT NOT NULL DEFAULT '',
    started_at   TEXT,
    finished_at  TEXT,
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS job_logs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id     TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    line       TEXT NOT NULL,
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS backup_versions (
    id          TEXT PRIMARY KEY,
    target_type TEXT NOT NULL,
    target_id   TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    bytes       INTEGER NOT NULL DEFAULT 0,
    path_on_disk TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_jobs_target ON jobs(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_versions_target ON backup_versions(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_job_logs_job ON job_logs(job_id);
