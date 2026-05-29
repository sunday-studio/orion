ALTER TABLE core_monitor_configs
  ADD COLUMN heartbeat_token_hash TEXT NOT NULL DEFAULT '';

ALTER TABLE core_monitor_configs
  ADD COLUMN last_signal_at DATETIME;

CREATE INDEX IF NOT EXISTS idx_core_monitor_configs_heartbeat_token_hash
  ON core_monitor_configs(heartbeat_token_hash);
