CREATE TABLE core_worker_statuses (
    worker_id TEXT PRIMARY KEY,
    process_kind TEXT NOT NULL DEFAULT 'core-monitor-worker',
    hostname TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'unknown',
    version TEXT NOT NULL DEFAULT '',
    started_at DATETIME NOT NULL,
    last_heartbeat_at DATETIME NOT NULL,
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX idx_core_worker_statuses_last_heartbeat_at
    ON core_worker_statuses(last_heartbeat_at);
