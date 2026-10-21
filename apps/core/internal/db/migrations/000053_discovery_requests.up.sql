CREATE TABLE IF NOT EXISTS agent_discovery_requests (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  type TEXT NOT NULL,
  payload TEXT NOT NULL DEFAULT '{}',
  status TEXT NOT NULL,
  result TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  delivered_at DATETIME,
  finished_at DATETIME,
  expires_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_discovery_requests_agent ON agent_discovery_requests(agent_id, created_at);
ALTER TABLE agent_discoveries ADD COLUMN capabilities TEXT NOT NULL DEFAULT '[]';
ALTER TABLE agent_monitor_definitions ADD COLUMN delivered_at DATETIME;
ALTER TABLE agent_monitor_definitions ADD COLUMN failure_reason TEXT NOT NULL DEFAULT '';
