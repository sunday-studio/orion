ALTER TABLE monitors ADD COLUMN maintenance_starts_at DATETIME;
ALTER TABLE monitors ADD COLUMN maintenance_ends_at DATETIME;
ALTER TABLE monitors ADD COLUMN maintenance_reason TEXT NOT NULL DEFAULT '';
