ALTER TABLE status_pages ADD COLUMN custom_domain TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_status_pages_custom_domain
    ON status_pages(custom_domain)
    WHERE custom_domain <> '';
