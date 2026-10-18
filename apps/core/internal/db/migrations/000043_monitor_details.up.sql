ALTER TABLE monitors ADD COLUMN runbook_url TEXT NOT NULL DEFAULT '';
ALTER TABLE core_monitor_configs ADD COLUMN paused_at DATETIME;
ALTER TABLE core_monitor_configs ADD COLUMN paused_by TEXT NOT NULL DEFAULT '';
ALTER TABLE core_monitor_configs ADD COLUMN heartbeat_token_prefix TEXT NOT NULL DEFAULT '';
