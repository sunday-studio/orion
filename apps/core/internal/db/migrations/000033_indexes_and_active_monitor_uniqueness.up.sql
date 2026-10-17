-- Orphaned attempts would violate the alert_delivery_attempts foreign key that
-- Core now enforces (PRAGMA foreign_keys=ON).
DELETE FROM alert_delivery_attempts
WHERE alert_delivery_id NOT IN (SELECT id FROM alert_deliveries);

-- Keep the oldest non-deleted monitor per (agent, name); soft-delete the rest so the
-- unique index below can be created. Their history stays queryable.
UPDATE monitors
SET lifecycle = 'deleted',
    deleted_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP,
    active_incident_id = ''
WHERE lifecycle <> 'deleted'
  AND id NOT IN (
    SELECT id FROM (
      SELECT id,
             ROW_NUMBER() OVER (PARTITION BY agent_id, name ORDER BY created_at ASC, id ASC) AS rn
      FROM monitors
      WHERE lifecycle <> 'deleted'
    ) WHERE rn = 1
  );

CREATE UNIQUE INDEX IF NOT EXISTS idx_monitors_agent_name_active
  ON monitors(agent_id, name)
  WHERE lifecycle <> 'deleted';

CREATE INDEX IF NOT EXISTS idx_monitor_reports_monitor_created
  ON monitor_reports(monitor_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_reports_agent_created
  ON agent_reports(agent_id, created_at DESC);
