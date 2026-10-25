CREATE TABLE IF NOT EXISTS log_saved_views (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  filters_json TEXT NOT NULL DEFAULT '{}',
  created_by TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL
);
