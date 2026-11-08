-- Retention for resource and app metric samples, which live in metrics.db (not in this database).
-- Raw one-minute samples are kept 7 days and hourly rollups 90 days unless changed.
ALTER TABLE data_lifecycle_settings ADD COLUMN metric_sample_retention_days INTEGER NOT NULL DEFAULT 7;
ALTER TABLE data_lifecycle_settings ADD COLUMN metric_rollup_retention_days INTEGER NOT NULL DEFAULT 90;
