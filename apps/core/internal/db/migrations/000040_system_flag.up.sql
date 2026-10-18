ALTER TABLE agents ADD COLUMN system BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE monitors ADD COLUMN system BOOLEAN NOT NULL DEFAULT 0;
UPDATE agents SET system = 1 WHERE machine_id = 'core';
UPDATE monitors SET system = 1 WHERE id = 'monitor-core-worker-liveness' OR name LIKE 'Agent connectivity: %';
