CREATE TABLE IF NOT EXISTS manifest_files (
  version_id TEXT NOT NULL,
  path TEXT NOT NULL,
  size INTEGER NOT NULL DEFAULT 0,
  is_dir INTEGER NOT NULL DEFAULT 0,
  mtime TEXT,
  PRIMARY KEY (version_id, path),
  FOREIGN KEY (version_id) REFERENCES backup_versions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_manifest_files_version_path ON manifest_files(version_id, path);
