-- Keep only the newest active incident per monitor, then enforce it.
UPDATE incidents
SET status = 'resolved',
    resolved_at = CURRENT_TIMESTAMP,
    resolution_kind = 'deduplicated',
    latest_event = 'Resolved as duplicate of a newer active incident'
WHERE status IN ('open', 'acknowledged', 'covered')
  AND monitor_id <> ''
  AND id NOT IN (
    SELECT id FROM (
      SELECT id,
             ROW_NUMBER() OVER (PARTITION BY monitor_id ORDER BY opened_at DESC, id DESC) AS rn
      FROM incidents
      WHERE status IN ('open', 'acknowledged', 'covered') AND monitor_id <> ''
    ) WHERE rn = 1
  );

CREATE UNIQUE INDEX IF NOT EXISTS idx_incidents_one_active_per_monitor
  ON incidents(monitor_id)
  WHERE status IN ('open', 'acknowledged', 'covered') AND monitor_id <> '';
