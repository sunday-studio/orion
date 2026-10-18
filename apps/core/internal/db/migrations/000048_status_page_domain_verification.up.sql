ALTER TABLE status_pages ADD COLUMN domain_status TEXT NOT NULL DEFAULT 'pending';
ALTER TABLE status_pages ADD COLUMN domain_checked_at DATETIME;
ALTER TABLE status_pages ADD COLUMN domain_verified_at DATETIME;
ALTER TABLE status_pages ADD COLUMN domain_error TEXT NOT NULL DEFAULT '';
