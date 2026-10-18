ALTER TABLE alert_routes ADD COLUMN cooldown_seconds INTEGER NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS alert_label_routes (
  id TEXT PRIMARY KEY,
  position INTEGER NOT NULL DEFAULT 0,
  name TEXT NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT 1,
  matchers TEXT NOT NULL DEFAULT '[]',
  channel_ids TEXT NOT NULL DEFAULT '[]',
  continue_matching BOOLEAN NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_alert_label_routes_position ON alert_label_routes(position);
