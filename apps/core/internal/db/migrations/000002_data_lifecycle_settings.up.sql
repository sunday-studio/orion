CREATE TABLE IF NOT EXISTS data_lifecycle_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    raw_report_hot_days INTEGER NOT NULL,
    archive_raw_reports BOOLEAN NOT NULL,
    archive_dir TEXT NOT NULL,
    rollups_enabled BOOLEAN NOT NULL,
    rollup_retention_days INTEGER,
    archive_schedule TEXT NOT NULL,
    last_rollup_run_at DATETIME,
    last_archive_run_at DATETIME,
    last_archive_status TEXT,
    last_archive_error TEXT,
    created_at DATETIME,
    updated_at DATETIME
);
