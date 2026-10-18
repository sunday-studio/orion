ALTER TABLE alert_delivery_attempts ADD COLUMN http_status INTEGER NOT NULL DEFAULT 0;
ALTER TABLE alert_delivery_attempts ADD COLUMN response_snippet TEXT NOT NULL DEFAULT '';
ALTER TABLE alert_channels ADD COLUMN headers_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE alert_channels ADD COLUMN email_delivery_mode TEXT NOT NULL DEFAULT 'instant';
ALTER TABLE alert_channels ADD COLUMN email_subject_prefix TEXT NOT NULL DEFAULT '';
