ALTER TABLE monitors ADD COLUMN active_incident_id TEXT NOT NULL DEFAULT '';
ALTER TABLE monitors ADD COLUMN incident_state TEXT NOT NULL DEFAULT 'unknown';

CREATE INDEX IF NOT EXISTS idx_monitors_active_incident_id
  ON monitors(active_incident_id);

CREATE INDEX IF NOT EXISTS idx_monitors_incident_state
  ON monitors(incident_state);

CREATE INDEX IF NOT EXISTS idx_incidents_monitor_status_opened
  ON incidents(monitor_id, status, opened_at);
