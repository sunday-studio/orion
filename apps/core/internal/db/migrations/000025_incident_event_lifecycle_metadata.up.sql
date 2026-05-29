CREATE TABLE IF NOT EXISTS incident_events (
  id VARCHAR(255) PRIMARY KEY,
  incident_id TEXT NOT NULL,
  type TEXT NOT NULL,
  message TEXT,
  monitor_report_id TEXT,
  created_at DATETIME
);

ALTER TABLE incident_events ADD COLUMN actor_type TEXT NOT NULL DEFAULT 'system';
ALTER TABLE incident_events ADD COLUMN actor_id TEXT NOT NULL DEFAULT 'core';
ALTER TABLE incident_events ADD COLUMN note TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_incident_events_incident_id
  ON incident_events(incident_id);
CREATE INDEX IF NOT EXISTS idx_incident_events_monitor_report_id
  ON incident_events(monitor_report_id);
CREATE INDEX IF NOT EXISTS idx_incident_events_created_at
  ON incident_events(created_at);
CREATE INDEX IF NOT EXISTS idx_incident_events_actor
  ON incident_events(actor_type, actor_id);
