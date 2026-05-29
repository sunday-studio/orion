ALTER TABLE agents ADD COLUMN token_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN token_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE agents ADD COLUMN token_rotated_at DATETIME;
ALTER TABLE agents ADD COLUMN token_revoked_at DATETIME;
ALTER TABLE agents ADD COLUMN token_revocation_reason TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_agents_token_hash ON agents(token_hash);
CREATE INDEX IF NOT EXISTS idx_agents_token_revoked_at ON agents(token_revoked_at);

ALTER TABLE audit_events ADD COLUMN metadata_json TEXT NOT NULL DEFAULT '{}';
