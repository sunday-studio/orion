ALTER TABLE monitor_reports ADD COLUMN client_report_id TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_monitor_reports_client_report_id
  ON monitor_reports(monitor_id, client_report_id)
  WHERE client_report_id <> '';
