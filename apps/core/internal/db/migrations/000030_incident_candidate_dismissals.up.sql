CREATE TABLE IF NOT EXISTS incident_candidate_dismissals (
  monitor_id TEXT PRIMARY KEY,
  dismissed_by TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  dismissed_at DATETIME NOT NULL
);
