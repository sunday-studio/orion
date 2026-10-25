CREATE TABLE IF NOT EXISTS agent_discoveries (
  agent_id TEXT PRIMARY KEY,
  scanned_at DATETIME,
  enabled INTEGER NOT NULL DEFAULT 1,
  kinds TEXT NOT NULL DEFAULT '[]',
  items TEXT NOT NULL DEFAULT '[]',
  received_at DATETIME NOT NULL
);
