CREATE TABLE IF NOT EXISTS log_alert_rules (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  app_id TEXT NOT NULL,
  mode TEXT NOT NULL DEFAULT 'threshold',
  severity TEXT NOT NULL DEFAULT '',
  event TEXT NOT NULL DEFAULT '',
  text TEXT NOT NULL DEFAULT '',
  threshold INTEGER NOT NULL DEFAULT 0,
  window_seconds INTEGER NOT NULL DEFAULT 300,
  enabled BOOLEAN NOT NULL DEFAULT 1,
  monitor_id TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL DEFAULT 'ok',
  last_count INTEGER NOT NULL DEFAULT 0,
  last_evaluated_at DATETIME,
  last_reported_at DATETIME,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  FOREIGN KEY (app_id) REFERENCES apps(id)
);
CREATE INDEX IF NOT EXISTS idx_log_alert_rules_app_id ON log_alert_rules(app_id);
