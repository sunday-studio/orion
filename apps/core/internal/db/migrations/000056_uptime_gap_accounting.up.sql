-- Time-based uptime: rollups also record the checks that should have run but did not (missed_count) and the
-- monitor's check interval, so uptime can be weighted by time and silence counts as downtime.
-- Existing rollups keep missed_count = 0, so their history reads exactly as before.
ALTER TABLE monitor_uptime_rollups ADD COLUMN missed_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE monitor_uptime_rollups ADD COLUMN interval_seconds INTEGER NOT NULL DEFAULT 0;

-- Periods when a monitor was not expected to report: paused, in a maintenance window, or its node was in
-- maintenance. These are kept as history because the monitor and node rows only hold the current state.
-- Triggers write them so every code path that pauses or schedules maintenance is covered.
CREATE TABLE IF NOT EXISTS uptime_exclusions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  subject_type TEXT NOT NULL,   -- monitor_pause | monitor_maintenance | node_maintenance
  subject_id TEXT NOT NULL,
  starts_at DATETIME NOT NULL,
  ends_at DATETIME              -- NULL while open
);
CREATE INDEX IF NOT EXISTS idx_uptime_exclusions_subject ON uptime_exclusions(subject_type, subject_id, starts_at);
CREATE INDEX IF NOT EXISTS idx_uptime_exclusions_ends_at ON uptime_exclusions(ends_at);

-- Open periods for state that is already active when this migration runs.
INSERT INTO uptime_exclusions (subject_type, subject_id, starts_at, ends_at)
SELECT 'monitor_pause', monitor_id, COALESCE(paused_at, strftime('%Y-%m-%d %H:%M:%f+00:00', 'now')), NULL
FROM core_monitor_configs WHERE paused = 1;

INSERT INTO uptime_exclusions (subject_type, subject_id, starts_at, ends_at)
SELECT 'monitor_maintenance', id, COALESCE(maintenance_starts_at, strftime('%Y-%m-%d %H:%M:%f+00:00', 'now')), maintenance_ends_at
FROM monitors WHERE maintenance_ends_at IS NOT NULL;

INSERT INTO uptime_exclusions (subject_type, subject_id, starts_at, ends_at)
SELECT 'node_maintenance', id, strftime('%Y-%m-%d %H:%M:%f+00:00', 'now'), maintenance_until
FROM nodes WHERE maintenance_mode = 1;

CREATE TRIGGER trg_uptime_exclusions_pause_on AFTER UPDATE OF paused ON core_monitor_configs
WHEN NEW.paused = 1 AND OLD.paused = 0
BEGIN
  INSERT INTO uptime_exclusions (subject_type, subject_id, starts_at, ends_at)
  VALUES ('monitor_pause', NEW.monitor_id, strftime('%Y-%m-%d %H:%M:%f+00:00', 'now'), NULL);
END;

CREATE TRIGGER trg_uptime_exclusions_pause_off AFTER UPDATE OF paused ON core_monitor_configs
WHEN NEW.paused = 0 AND OLD.paused = 1
BEGIN
  UPDATE uptime_exclusions SET ends_at = strftime('%Y-%m-%d %H:%M:%f+00:00', 'now')
  WHERE subject_type = 'monitor_pause' AND subject_id = NEW.monitor_id AND ends_at IS NULL;
END;

CREATE TRIGGER trg_uptime_exclusions_monitor_maintenance AFTER UPDATE OF maintenance_starts_at, maintenance_ends_at ON monitors
WHEN COALESCE(OLD.maintenance_starts_at, '') <> COALESCE(NEW.maintenance_starts_at, '')
  OR COALESCE(OLD.maintenance_ends_at, '') <> COALESCE(NEW.maintenance_ends_at, '')
BEGIN
  -- Cut a window that is running now, and drop ones that had not started: the new state replaces them.
  UPDATE uptime_exclusions SET ends_at = strftime('%Y-%m-%d %H:%M:%f+00:00', 'now')
  WHERE subject_type = 'monitor_maintenance' AND subject_id = NEW.id
    AND starts_at <= strftime('%Y-%m-%d %H:%M:%f+00:00', 'now')
    AND (ends_at IS NULL OR ends_at > strftime('%Y-%m-%d %H:%M:%f+00:00', 'now'));
  DELETE FROM uptime_exclusions
  WHERE subject_type = 'monitor_maintenance' AND subject_id = NEW.id
    AND starts_at > strftime('%Y-%m-%d %H:%M:%f+00:00', 'now');
  INSERT INTO uptime_exclusions (subject_type, subject_id, starts_at, ends_at)
  SELECT 'monitor_maintenance', NEW.id, COALESCE(NEW.maintenance_starts_at, strftime('%Y-%m-%d %H:%M:%f+00:00', 'now')), NEW.maintenance_ends_at
  WHERE NEW.maintenance_ends_at IS NOT NULL;
END;

CREATE TRIGGER trg_uptime_exclusions_node_maintenance AFTER UPDATE OF maintenance_mode, maintenance_until ON nodes
WHEN OLD.maintenance_mode <> NEW.maintenance_mode
  OR COALESCE(OLD.maintenance_until, '') <> COALESCE(NEW.maintenance_until, '')
BEGIN
  UPDATE uptime_exclusions SET ends_at = strftime('%Y-%m-%d %H:%M:%f+00:00', 'now')
  WHERE subject_type = 'node_maintenance' AND subject_id = NEW.id
    AND (ends_at IS NULL OR ends_at > strftime('%Y-%m-%d %H:%M:%f+00:00', 'now'));
  INSERT INTO uptime_exclusions (subject_type, subject_id, starts_at, ends_at)
  SELECT 'node_maintenance', NEW.id, strftime('%Y-%m-%d %H:%M:%f+00:00', 'now'), NEW.maintenance_until
  WHERE NEW.maintenance_mode = 1;
END;
