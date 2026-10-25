CREATE TABLE IF NOT EXISTS service_log_entries (
    id VARCHAR(255) PRIMARY KEY,
    agent_id TEXT NOT NULL,
    monitor_id TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'agent',
    stream TEXT NOT NULL DEFAULT 'jsonl',
    level TEXT NOT NULL DEFAULT 'INFO',
    component TEXT NOT NULL DEFAULT '',
    monitor_name TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    fields_json TEXT NOT NULL DEFAULT '{}',
    raw TEXT NOT NULL DEFAULT '',
    fingerprint TEXT NOT NULL,
    occurred_at DATETIME NOT NULL,
    collected_at DATETIME NOT NULL,
    created_at DATETIME
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_service_log_entries_agent_fingerprint ON service_log_entries(agent_id, fingerprint);
CREATE INDEX IF NOT EXISTS idx_service_log_entries_agent_time ON service_log_entries(agent_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_service_log_entries_monitor_time ON service_log_entries(monitor_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_service_log_entries_source_time ON service_log_entries(source, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_service_log_entries_level_time ON service_log_entries(level, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_service_log_entries_component_time ON service_log_entries(component, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_service_log_entries_collected_at ON service_log_entries(collected_at DESC);
CREATE INDEX IF NOT EXISTS idx_service_log_entries_created_at ON service_log_entries(created_at DESC);
