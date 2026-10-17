CREATE TABLE IF NOT EXISTS enrollment_sessions (
  id TEXT PRIMARY KEY,
  token_hash TEXT NOT NULL,
  platform TEXT NOT NULL DEFAULT '',
  machine_id TEXT NOT NULL DEFAULT '',
  agent_id TEXT NOT NULL DEFAULT '',
  claimed_at DATETIME,
  cancelled_at DATETIME,
  expires_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_enrollment_sessions_token_hash ON enrollment_sessions(token_hash);
