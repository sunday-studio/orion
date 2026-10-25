ALTER TABLE apps ADD COLUMN rate_limit_per_second INTEGER NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS log_exports (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL DEFAULT 'logs',
  format TEXT NOT NULL DEFAULT 'csv',
  filters_json TEXT NOT NULL DEFAULT '{}',
  include_raw INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'queued',
  rows INTEGER NOT NULL DEFAULT 0,
  bytes INTEGER NOT NULL DEFAULT 0,
  file_name TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  finished_at DATETIME,
  expires_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_log_exports_expires ON log_exports (expires_at);
