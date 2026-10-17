ALTER TABLE agents ADD COLUMN maintenance_until DATETIME;

-- Maintenance that was switched on before expiry existed ends 24h after this upgrade
-- instead of silencing the agent forever.
UPDATE agents
SET maintenance_until = datetime('now', '+24 hours')
WHERE maintenance_mode = 1 AND maintenance_until IS NULL;
