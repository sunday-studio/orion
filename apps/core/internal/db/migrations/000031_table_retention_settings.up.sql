ALTER TABLE data_lifecycle_settings ADD COLUMN service_log_retention_days INTEGER NOT NULL DEFAULT 14;
ALTER TABLE data_lifecycle_settings ADD COLUMN alert_delivery_retention_days INTEGER NOT NULL DEFAULT 90;
ALTER TABLE data_lifecycle_settings ADD COLUMN incident_event_retention_days INTEGER NOT NULL DEFAULT 365;
ALTER TABLE data_lifecycle_settings ADD COLUMN audit_event_retention_days INTEGER NOT NULL DEFAULT 365;
ALTER TABLE data_lifecycle_settings ADD COLUMN worker_status_retention_days INTEGER NOT NULL DEFAULT 7;
