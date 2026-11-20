-- Instance-wide ceiling on how long spans are kept (days). An app's own retention_days applies when it is shorter.
-- The spans themselves live in traces.db (not in this database).
ALTER TABLE data_lifecycle_settings ADD COLUMN trace_retention_days INTEGER NOT NULL DEFAULT 7;
