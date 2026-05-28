ALTER TABLE incidents ADD COLUMN covered_at DATETIME;
ALTER TABLE incidents ADD COLUMN covered_until DATETIME;
ALTER TABLE incidents ADD COLUMN coverage_note TEXT;
ALTER TABLE incidents ADD COLUMN resolution_kind TEXT NOT NULL DEFAULT '';
ALTER TABLE incidents ADD COLUMN reopened_at DATETIME;
ALTER TABLE incidents ADD COLUMN reopen_count INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_incidents_covered_until
  ON incidents(covered_until);
