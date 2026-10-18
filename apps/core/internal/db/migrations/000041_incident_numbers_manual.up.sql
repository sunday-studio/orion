ALTER TABLE incidents ADD COLUMN number INTEGER NOT NULL DEFAULT 0;
ALTER TABLE incidents ADD COLUMN responder TEXT NOT NULL DEFAULT '';
ALTER TABLE incidents ADD COLUMN responder_at DATETIME;
ALTER TABLE incidents ADD COLUMN summary TEXT NOT NULL DEFAULT '';
ALTER TABLE incidents ADD COLUMN source TEXT NOT NULL DEFAULT 'monitor';
ALTER TABLE incidents ADD COLUMN covered_by_incident_id TEXT NOT NULL DEFAULT '';
UPDATE incidents SET number = (
  SELECT COUNT(*) FROM incidents i2
  WHERE i2.opened_at < incidents.opened_at OR (i2.opened_at = incidents.opened_at AND i2.id <= incidents.id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_incidents_number ON incidents(number) WHERE number > 0;
