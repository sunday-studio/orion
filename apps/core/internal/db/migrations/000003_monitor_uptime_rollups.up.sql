CREATE TABLE IF NOT EXISTS monitor_uptime_rollups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    monitor_id TEXT NOT NULL,
    date TEXT NOT NULL,
    up_count INTEGER NOT NULL DEFAULT 0,
    down_count INTEGER NOT NULL DEFAULT 0,
    degraded_count INTEGER NOT NULL DEFAULT 0,
    unknown_count INTEGER NOT NULL DEFAULT 0,
    total_count INTEGER NOT NULL DEFAULT 0,
    uptime_percent REAL NOT NULL DEFAULT 0,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_monitor_uptime_rollups_monitor_date ON monitor_uptime_rollups(monitor_id, date);
CREATE INDEX IF NOT EXISTS idx_monitor_uptime_rollups_date ON monitor_uptime_rollups(date);
