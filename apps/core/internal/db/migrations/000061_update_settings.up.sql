-- Update checks: whether Core looks for a newer release, which channel, and what the last look found.
ALTER TABLE instance_settings ADD COLUMN update_check_enabled BOOLEAN NOT NULL DEFAULT 1;
ALTER TABLE instance_settings ADD COLUMN update_channel TEXT NOT NULL DEFAULT 'stable';
ALTER TABLE instance_settings ADD COLUMN update_latest_version TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN update_latest_released_at DATETIME;
ALTER TABLE instance_settings ADD COLUMN update_latest_security BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE instance_settings ADD COLUMN update_latest_url TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN update_checked_at DATETIME;
ALTER TABLE instance_settings ADD COLUMN update_check_error TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN update_skipped_version TEXT NOT NULL DEFAULT '';
