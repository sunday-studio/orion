ALTER TABLE alert_deliveries ADD COLUMN attempt_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE alert_deliveries ADD COLUMN max_attempts INTEGER NOT NULL DEFAULT 3;
ALTER TABLE alert_deliveries ADD COLUMN next_attempt_at DATETIME;
ALTER TABLE alert_deliveries ADD COLUMN last_attempt_at DATETIME;

CREATE INDEX IF NOT EXISTS idx_alert_deliveries_next_attempt_at ON alert_deliveries(next_attempt_at);

CREATE TABLE IF NOT EXISTS alert_delivery_attempts (
    id VARCHAR(255) PRIMARY KEY,
    alert_delivery_id TEXT NOT NULL,
    attempt_number INTEGER NOT NULL,
    status TEXT NOT NULL,
    stage TEXT NOT NULL,
    error TEXT,
    started_at DATETIME NOT NULL,
    completed_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME,
    FOREIGN KEY (alert_delivery_id) REFERENCES alert_deliveries(id)
);

CREATE INDEX IF NOT EXISTS idx_alert_delivery_attempts_delivery_id ON alert_delivery_attempts(alert_delivery_id);
CREATE INDEX IF NOT EXISTS idx_alert_delivery_attempts_delivery_number ON alert_delivery_attempts(attempt_number);
CREATE INDEX IF NOT EXISTS idx_alert_delivery_attempts_started_at ON alert_delivery_attempts(started_at);
