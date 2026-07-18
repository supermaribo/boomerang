ALTER TABLE backup_versions ADD COLUMN verified_at TEXT;
ALTER TABLE backup_versions ADD COLUMN verify_error TEXT NOT NULL DEFAULT '';
