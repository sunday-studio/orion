ALTER TABLE alert_channels ADD COLUMN subscribed_events TEXT NOT NULL DEFAULT '["incident_opened","incident_resolved"]';
