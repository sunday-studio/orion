CREATE TABLE IF NOT EXISTS alert_channels (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    webhook_url TEXT,
    email_to TEXT,
    email_from TEXT,
    smtp_host TEXT,
    smtp_port INTEGER NOT NULL DEFAULT 0,
    smtp_username TEXT,
    smtp_password TEXT,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_alert_channels_type ON alert_channels(type);
CREATE INDEX IF NOT EXISTS idx_alert_channels_enabled ON alert_channels(enabled);
