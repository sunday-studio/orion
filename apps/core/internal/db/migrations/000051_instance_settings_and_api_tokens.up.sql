CREATE TABLE IF NOT EXISTS instance_settings (
  id INTEGER PRIMARY KEY,
  instance_name TEXT NOT NULL DEFAULT '',
  public_url TEXT NOT NULL DEFAULT '',
  timezone TEXT NOT NULL DEFAULT 'UTC',
  time_format TEXT NOT NULL DEFAULT '24',
  registration_token TEXT NOT NULL DEFAULT '',
  registration_token_rotated_at DATETIME,
  updated_by TEXT NOT NULL DEFAULT '',
  updated_at DATETIME
);
CREATE TABLE IF NOT EXISTS api_tokens (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  prefix TEXT NOT NULL,
  token_hash TEXT NOT NULL,
  scope TEXT NOT NULL DEFAULT 'read',
  created_by TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  last_used_at DATETIME,
  revoked_at DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_tokens_hash ON api_tokens (token_hash);
CREATE INDEX IF NOT EXISTS idx_api_tokens_name ON api_tokens (name);
