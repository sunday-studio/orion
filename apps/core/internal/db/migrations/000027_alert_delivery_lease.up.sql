ALTER TABLE alert_deliveries ADD COLUMN locked_by TEXT NOT NULL DEFAULT '';
ALTER TABLE alert_deliveries ADD COLUMN locked_until DATETIME;

CREATE INDEX IF NOT EXISTS idx_alert_deliveries_status_next_attempt
  ON alert_deliveries(status, next_attempt_at);
