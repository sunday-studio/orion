CREATE TABLE IF NOT EXISTS agent_monitor_definitions (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  name TEXT NOT NULL,
  check_type TEXT NOT NULL,
  params TEXT NOT NULL DEFAULT '{}',
  interval_seconds INTEGER NOT NULL DEFAULT 30,
  fail_after INTEGER NOT NULL DEFAULT 3,
  created_by TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_monitor_definitions_name ON agent_monitor_definitions(agent_id, name);
