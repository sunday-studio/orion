CREATE TABLE IF NOT EXISTS alert_routes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    priority INTEGER NOT NULL DEFAULT 100,
    event_types TEXT,
    severities TEXT,
    agent_ids TEXT,
    monitor_ids TEXT,
    monitor_types TEXT,
    channel_ids TEXT,
    suppress BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_alert_routes_enabled ON alert_routes(enabled);
CREATE INDEX IF NOT EXISTS idx_alert_routes_priority ON alert_routes(priority);
CREATE INDEX IF NOT EXISTS idx_alert_routes_suppress ON alert_routes(suppress);

ALTER TABLE alert_deliveries ADD COLUMN route_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_alert_deliveries_route_id ON alert_deliveries(route_id);
