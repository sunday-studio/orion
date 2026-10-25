ALTER TABLE alert_routes ADD COLUMN grouping_policy TEXT NOT NULL DEFAULT 'suppress';
ALTER TABLE alert_routes ADD COLUMN grouping_delay_seconds INTEGER NOT NULL DEFAULT 300;
CREATE INDEX IF NOT EXISTS idx_alert_routes_grouping_policy ON alert_routes(grouping_policy);
