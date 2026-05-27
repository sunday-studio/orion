CREATE TABLE IF NOT EXISTS alert_groups (
    id VARCHAR(255) PRIMARY KEY,
    group_key TEXT NOT NULL,
    status TEXT NOT NULL,
    event_type TEXT NOT NULL,
    severity TEXT NOT NULL,
    summary TEXT,
    first_incident_id TEXT NOT NULL,
    last_incident_id TEXT NOT NULL,
    incident_count INTEGER NOT NULL DEFAULT 0,
    first_event_at DATETIME NOT NULL,
    last_event_at DATETIME NOT NULL,
    resolved_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_alert_groups_group_key_status ON alert_groups(group_key, status);
CREATE INDEX IF NOT EXISTS idx_alert_groups_status ON alert_groups(status);
CREATE INDEX IF NOT EXISTS idx_alert_groups_severity ON alert_groups(severity);
CREATE INDEX IF NOT EXISTS idx_alert_groups_last_event_at ON alert_groups(last_event_at);

CREATE TABLE IF NOT EXISTS alert_group_members (
    id VARCHAR(255) PRIMARY KEY,
    alert_group_id TEXT NOT NULL,
    incident_id TEXT NOT NULL,
    created_at DATETIME,
    UNIQUE(alert_group_id, incident_id)
);

CREATE INDEX IF NOT EXISTS idx_alert_group_members_incident_id ON alert_group_members(incident_id);

ALTER TABLE alert_deliveries ADD COLUMN alert_group_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_alert_deliveries_alert_group_id ON alert_deliveries(alert_group_id);
