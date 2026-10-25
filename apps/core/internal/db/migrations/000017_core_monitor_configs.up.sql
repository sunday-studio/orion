CREATE TABLE IF NOT EXISTS core_monitor_configs (
    monitor_id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    config_json TEXT NOT NULL DEFAULT '{}',
    secret_ref_json TEXT NOT NULL DEFAULT '{}',
    interval_seconds INTEGER NOT NULL DEFAULT 60,
    timeout_seconds INTEGER NOT NULL DEFAULT 10,
    confirmation_period_seconds INTEGER NOT NULL DEFAULT 0,
    recovery_period_seconds INTEGER NOT NULL DEFAULT 0,
    paused BOOLEAN NOT NULL DEFAULT false,
    next_run_at DATETIME NOT NULL,
    last_run_at DATETIME,
    last_success_at DATETIME,
    last_failure_at DATETIME,
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_expires_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_core_monitor_configs_kind
  ON core_monitor_configs(kind);

CREATE INDEX IF NOT EXISTS idx_core_monitor_configs_due
  ON core_monitor_configs(paused, next_run_at, lease_expires_at);

CREATE INDEX IF NOT EXISTS idx_core_monitor_configs_lease_owner
  ON core_monitor_configs(lease_owner);

CREATE INDEX IF NOT EXISTS idx_core_monitor_configs_lease_expires_at
  ON core_monitor_configs(lease_expires_at);
