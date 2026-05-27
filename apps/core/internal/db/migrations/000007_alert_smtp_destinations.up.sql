CREATE TABLE IF NOT EXISTS alert_smtp_services (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    host TEXT NOT NULL,
    port INTEGER NOT NULL DEFAULT 0,
    username TEXT,
    password TEXT,
    from_email TEXT NOT NULL,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS alert_email_destinations (
    id TEXT PRIMARY KEY,
    smtp_service_id TEXT NOT NULL,
    name TEXT NOT NULL UNIQUE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    email_to TEXT NOT NULL,
    subscribed_events TEXT NOT NULL DEFAULT '["incident_opened","incident_resolved"]',
    created_at DATETIME,
    updated_at DATETIME,
    FOREIGN KEY (smtp_service_id) REFERENCES alert_smtp_services(id)
);

CREATE INDEX IF NOT EXISTS idx_alert_smtp_services_enabled ON alert_smtp_services(enabled);
CREATE INDEX IF NOT EXISTS idx_alert_email_destinations_smtp_service_id ON alert_email_destinations(smtp_service_id);
CREATE INDEX IF NOT EXISTS idx_alert_email_destinations_enabled ON alert_email_destinations(enabled);
