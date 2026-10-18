CREATE TABLE IF NOT EXISTS status_page_maintenance (
  id TEXT PRIMARY KEY,
  status_page_id TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  component_ids TEXT NOT NULL DEFAULT '[]',
  hold_alerts INTEGER NOT NULL DEFAULT 0,
  notify_subscribers INTEGER NOT NULL DEFAULT 0,
  notice_sent_at DATETIME,
  starts_at DATETIME NOT NULL,
  ends_at DATETIME NOT NULL,
  created_at DATETIME,
  updated_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_status_page_maintenance_page ON status_page_maintenance (status_page_id);
CREATE INDEX IF NOT EXISTS idx_status_page_maintenance_window ON status_page_maintenance (starts_at, ends_at);
-- Custom domains configured before verification existed keep working.
UPDATE status_pages SET domain_status = 'verified', domain_verified_at = updated_at WHERE custom_domain <> '' AND domain_status = 'pending';
