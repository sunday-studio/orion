CREATE TABLE IF NOT EXISTS apps (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  slug TEXT NOT NULL,
  environment TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  retention_days INTEGER NOT NULL DEFAULT 14,
  max_storage_mb INTEGER NOT NULL DEFAULT 0,
  debug_sample_rate REAL NOT NULL DEFAULT 1,
  display_json TEXT NOT NULL DEFAULT '{}',
  redaction_json TEXT NOT NULL DEFAULT '{}',
  last_ingest_at DATETIME,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_apps_slug ON apps(slug);

CREATE TABLE IF NOT EXISTS app_ingest_keys (
  id TEXT PRIMARY KEY,
  app_id TEXT NOT NULL,
  name TEXT NOT NULL,
  prefix TEXT NOT NULL,
  key_hash TEXT NOT NULL,
  last_used_at DATETIME,
  revoked_at DATETIME,
  created_at DATETIME NOT NULL,
  FOREIGN KEY (app_id) REFERENCES apps(id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_app_ingest_keys_hash ON app_ingest_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_app_ingest_keys_app_id ON app_ingest_keys(app_id);
