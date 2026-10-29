-- Fixes to the uptime_exclusions triggers from 000056 (that migration is already applied, so it is not edited):
--   * a monitor config created already paused was never recorded;
--   * the node trigger compared a nullable column with <>, so NULL -> 1 did not fire;
--   * exclusions were never removed when their monitor or node was deleted.
CREATE TRIGGER IF NOT EXISTS trg_uptime_exclusions_pause_insert AFTER INSERT ON core_monitor_configs
WHEN NEW.paused = 1
BEGIN
  INSERT INTO uptime_exclusions (subject_type, subject_id, starts_at, ends_at)
  VALUES ('monitor_pause', NEW.monitor_id, strftime('%Y-%m-%d %H:%M:%f+00:00', 'now'), NULL);
END;

DROP TRIGGER IF EXISTS trg_uptime_exclusions_node_maintenance;
CREATE TRIGGER trg_uptime_exclusions_node_maintenance AFTER UPDATE OF maintenance_mode, maintenance_until ON nodes
WHEN COALESCE(OLD.maintenance_mode, 0) <> COALESCE(NEW.maintenance_mode, 0)
  OR COALESCE(OLD.maintenance_until, '') <> COALESCE(NEW.maintenance_until, '')
BEGIN
  UPDATE uptime_exclusions SET ends_at = strftime('%Y-%m-%d %H:%M:%f+00:00', 'now')
  WHERE subject_type = 'node_maintenance' AND subject_id = NEW.id
    AND (ends_at IS NULL OR ends_at > strftime('%Y-%m-%d %H:%M:%f+00:00', 'now'));
  INSERT INTO uptime_exclusions (subject_type, subject_id, starts_at, ends_at)
  SELECT 'node_maintenance', NEW.id, strftime('%Y-%m-%d %H:%M:%f+00:00', 'now'), NEW.maintenance_until
  WHERE COALESCE(NEW.maintenance_mode, 0) = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_uptime_exclusions_monitor_deleted AFTER DELETE ON monitors
BEGIN
  DELETE FROM uptime_exclusions WHERE subject_type IN ('monitor_pause', 'monitor_maintenance') AND subject_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS trg_uptime_exclusions_node_deleted AFTER DELETE ON nodes
BEGIN
  DELETE FROM uptime_exclusions WHERE subject_type = 'node_maintenance' AND subject_id = OLD.id;
END;

-- Rows left behind by deletions before these triggers existed.
DELETE FROM uptime_exclusions WHERE subject_type IN ('monitor_pause', 'monitor_maintenance') AND subject_id NOT IN (SELECT id FROM monitors);
DELETE FROM uptime_exclusions WHERE subject_type = 'node_maintenance' AND subject_id NOT IN (SELECT id FROM nodes);
