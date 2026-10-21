CREATE TABLE IF NOT EXISTS agent_app_resources (
  agent_id TEXT NOT NULL,
  key TEXT NOT NULL,
  kind TEXT NOT NULL,
  name TEXT NOT NULL,
  cpu_percent REAL,
  memory_bytes INTEGER,
  io_read_bytes_per_sec REAL,
  io_write_bytes_per_sec REAL,
  net_rx_bytes_per_sec REAL,
  net_tx_bytes_per_sec REAL,
  pids INTEGER,
  storage_bytes INTEGER,
  storage_capped INTEGER NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (agent_id, key)
);

CREATE TABLE IF NOT EXISTS agent_app_resource_reports (
  agent_id TEXT PRIMARY KEY,
  interval_seconds REAL NOT NULL DEFAULT 0,
  notes TEXT NOT NULL DEFAULT '[]',
  reported_at DATETIME NOT NULL
);
