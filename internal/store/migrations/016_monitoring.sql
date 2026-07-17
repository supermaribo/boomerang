CREATE TABLE IF NOT EXISTS monitored_servers (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL,
    host                TEXT NOT NULL,
    port                INTEGER NOT NULL DEFAULT 22,
    username            TEXT NOT NULL DEFAULT 'boomerang-monitor',
    enc_secret          BLOB,
    ssh_host_key        TEXT NOT NULL DEFAULT '',
    file_server_id      TEXT,
    enabled             INTEGER NOT NULL DEFAULT 1,
    poll_interval_sec   INTEGER NOT NULL DEFAULT 60,
    offline_after_sec   INTEGER NOT NULL DEFAULT 180,
    alert_cpu_percent   REAL NOT NULL DEFAULT 90,
    alert_mem_percent   REAL NOT NULL DEFAULT 90,
    alert_disk_percent  REAL NOT NULL DEFAULT 90,
    alert_load_per_cpu  REAL NOT NULL DEFAULT 2,
    alert_sustain_sec   INTEGER NOT NULL DEFAULT 300,
    alerts_enabled      INTEGER NOT NULL DEFAULT 1,
    client_version      TEXT NOT NULL DEFAULT '',
    last_sample_at      TEXT,
    last_poll_at        TEXT,
    last_poll_error     TEXT NOT NULL DEFAULT '',
    last_boot_id        TEXT NOT NULL DEFAULT '',
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (file_server_id) REFERENCES file_servers(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS monitor_samples (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id       TEXT NOT NULL,
    sampled_at      TEXT NOT NULL,
    boot_id         TEXT NOT NULL DEFAULT '',
    uptime_sec      INTEGER NOT NULL DEFAULT 0,
    cpu_percent     REAL NOT NULL DEFAULT 0,
    mem_total_bytes INTEGER NOT NULL DEFAULT 0,
    mem_used_bytes  INTEGER NOT NULL DEFAULT 0,
    mem_avail_bytes INTEGER NOT NULL DEFAULT 0,
    swap_total_bytes INTEGER NOT NULL DEFAULT 0,
    swap_used_bytes INTEGER NOT NULL DEFAULT 0,
    load1           REAL NOT NULL DEFAULT 0,
    load5           REAL NOT NULL DEFAULT 0,
    load15          REAL NOT NULL DEFAULT 0,
    num_cpu         INTEGER NOT NULL DEFAULT 0,
    client_version  TEXT NOT NULL DEFAULT '',
    UNIQUE(server_id, sampled_at),
    FOREIGN KEY (server_id) REFERENCES monitored_servers(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_monitor_samples_server_time
    ON monitor_samples(server_id, sampled_at DESC);

CREATE TABLE IF NOT EXISTS monitor_filesystems (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    sample_id       INTEGER NOT NULL,
    server_id       TEXT NOT NULL,
    sampled_at      TEXT NOT NULL,
    mount           TEXT NOT NULL,
    device          TEXT NOT NULL DEFAULT '',
    fs_type         TEXT NOT NULL DEFAULT '',
    total_bytes     INTEGER NOT NULL DEFAULT 0,
    used_bytes      INTEGER NOT NULL DEFAULT 0,
    free_bytes      INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (sample_id) REFERENCES monitor_samples(id) ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES monitored_servers(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_monitor_fs_server_time
    ON monitor_filesystems(server_id, sampled_at DESC);

CREATE TABLE IF NOT EXISTS monitor_hourly (
    server_id           TEXT NOT NULL,
    hour_at             TEXT NOT NULL,
    samples             INTEGER NOT NULL DEFAULT 0,
    avg_cpu_percent     REAL NOT NULL DEFAULT 0,
    max_cpu_percent     REAL NOT NULL DEFAULT 0,
    avg_mem_percent     REAL NOT NULL DEFAULT 0,
    max_mem_percent     REAL NOT NULL DEFAULT 0,
    avg_load1           REAL NOT NULL DEFAULT 0,
    max_load1           REAL NOT NULL DEFAULT 0,
    max_disk_percent    REAL NOT NULL DEFAULT 0,
    PRIMARY KEY (server_id, hour_at),
    FOREIGN KEY (server_id) REFERENCES monitored_servers(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS monitor_alert_state (
    server_id       TEXT NOT NULL,
    alert_key       TEXT NOT NULL,
    active          INTEGER NOT NULL DEFAULT 0,
    since_at        TEXT,
    last_sent_at    TEXT,
    last_value      TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (server_id, alert_key),
    FOREIGN KEY (server_id) REFERENCES monitored_servers(id) ON DELETE CASCADE
);
