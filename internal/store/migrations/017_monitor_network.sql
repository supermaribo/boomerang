ALTER TABLE monitor_samples ADD COLUMN net_iface TEXT NOT NULL DEFAULT '';
ALTER TABLE monitor_samples ADD COLUMN net_rx_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE monitor_samples ADD COLUMN net_tx_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE monitor_samples ADD COLUMN net_rx_bps REAL;
ALTER TABLE monitor_samples ADD COLUMN net_tx_bps REAL;

ALTER TABLE monitor_hourly ADD COLUMN avg_net_rx_bps REAL NOT NULL DEFAULT 0;
ALTER TABLE monitor_hourly ADD COLUMN max_net_rx_bps REAL NOT NULL DEFAULT 0;
ALTER TABLE monitor_hourly ADD COLUMN avg_net_tx_bps REAL NOT NULL DEFAULT 0;
ALTER TABLE monitor_hourly ADD COLUMN max_net_tx_bps REAL NOT NULL DEFAULT 0;
