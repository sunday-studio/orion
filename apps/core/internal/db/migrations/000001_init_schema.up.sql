CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS agents (
    id TEXT PRIMARY KEY,
    machine_id TEXT NOT NULL,
    name TEXT NOT NULL,
    os TEXT NOT NULL,
    platform TEXT,
    kernel_version TEXT,
    arch TEXT NOT NULL,
    token TEXT NOT NULL,
    maintenance_mode BOOLEAN DEFAULT false,
    reporting_interval_seconds INTEGER DEFAULT 60,
    created_at DATETIME,
    deleted_at DATETIME,
    last_seen DATETIME,
    location JSON,
    meta TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_machine_id ON agents(machine_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_token ON agents(token);

CREATE TABLE IF NOT EXISTS agent_reports (
    id VARCHAR(255) PRIMARY KEY,
    agent_id TEXT NOT NULL,
    created_at DATETIME,
    agent_version TEXT,
    config_summary TEXT,
    uptime_seconds INTEGER,
    timestamp TEXT,
    cpu JSON,
    memory JSON,
    disk JSON,
    location JSON
);

CREATE INDEX IF NOT EXISTS idx_agent_reports_agent_id ON agent_reports(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_reports_created_at ON agent_reports(created_at);

CREATE TABLE IF NOT EXISTS monitors (
    id TEXT PRIMARY KEY,
    description TEXT,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    last_successful_report_at DATETIME,
    reporting_interval_seconds INTEGER DEFAULT 60,
    computed_health TEXT DEFAULT 'unknown',
    last_health_computation DATETIME,
    lifecycle TEXT NOT NULL,
    health TEXT NOT NULL,
    meta TEXT,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_monitors_agent_id ON monitors(agent_id);
CREATE INDEX IF NOT EXISTS idx_monitors_lifecycle ON monitors(lifecycle);
CREATE INDEX IF NOT EXISTS idx_monitors_health ON monitors(health);
CREATE INDEX IF NOT EXISTS idx_monitors_created_at ON monitors(created_at);

CREATE TABLE IF NOT EXISTS monitor_reports (
    id VARCHAR(255) PRIMARY KEY,
    monitor_id TEXT NOT NULL,
    payload TEXT NOT NULL,
    collected_at TEXT NOT NULL,
    health TEXT NOT NULL,
    created_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_monitor_reports_monitor_id ON monitor_reports(monitor_id);
CREATE INDEX IF NOT EXISTS idx_monitor_reports_created_at ON monitor_reports(created_at);

CREATE TABLE IF NOT EXISTS incidents (
    id VARCHAR(255) PRIMARY KEY,
    status TEXT NOT NULL,
    severity TEXT NOT NULL,
    title TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    monitor_id TEXT NOT NULL,
    opened_at DATETIME NOT NULL,
    resolved_at DATETIME,
    last_event_at DATETIME NOT NULL,
    latest_event TEXT,
    notification_status TEXT NOT NULL DEFAULT 'pending',
    created_at DATETIME,
    updated_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status);
CREATE INDEX IF NOT EXISTS idx_incidents_severity ON incidents(severity);
CREATE INDEX IF NOT EXISTS idx_incidents_agent_id ON incidents(agent_id);
CREATE INDEX IF NOT EXISTS idx_incidents_monitor_id ON incidents(monitor_id);
CREATE INDEX IF NOT EXISTS idx_incidents_opened_at ON incidents(opened_at);

CREATE TABLE IF NOT EXISTS incident_events (
    id VARCHAR(255) PRIMARY KEY,
    incident_id TEXT NOT NULL,
    type TEXT NOT NULL,
    message TEXT,
    monitor_report_id TEXT,
    created_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_incident_events_incident_id ON incident_events(incident_id);
CREATE INDEX IF NOT EXISTS idx_incident_events_monitor_report_id ON incident_events(monitor_report_id);
CREATE INDEX IF NOT EXISTS idx_incident_events_created_at ON incident_events(created_at);

CREATE TABLE IF NOT EXISTS alert_deliveries (
    id VARCHAR(255) PRIMARY KEY,
    incident_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    channel TEXT NOT NULL,
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    error TEXT,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_alert_deliveries_incident_id ON alert_deliveries(incident_id);
CREATE INDEX IF NOT EXISTS idx_alert_deliveries_created_at ON alert_deliveries(created_at);
