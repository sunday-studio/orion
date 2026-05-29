CREATE TABLE IF NOT EXISTS status_page_subscribers (
    id VARCHAR(255) PRIMARY KEY,
    status_page_id TEXT NOT NULL,
    destination_type TEXT NOT NULL,
    destination_hash TEXT NOT NULL,
    destination_value_ciphertext TEXT NOT NULL DEFAULT '',
    masked_destination TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending',
    confirmation_token_hash TEXT NOT NULL DEFAULT '',
    confirmation_token_expires_at DATETIME,
    manage_token_hash TEXT NOT NULL DEFAULT '',
    manage_token_version INTEGER NOT NULL DEFAULT 1,
    unsubscribe_token_hash TEXT NOT NULL DEFAULT '',
    unsubscribe_token_version INTEGER NOT NULL DEFAULT 1,
    bounce_count INTEGER NOT NULL DEFAULT 0,
    last_delivery_status TEXT NOT NULL DEFAULT '',
    last_delivery_at DATETIME,
    source TEXT NOT NULL DEFAULT 'public_page',
    confirmed_at DATETIME,
    unsubscribed_at DATETIME,
    disabled_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_status_page_subscribers_destination
    ON status_page_subscribers(status_page_id, destination_type, destination_hash);
CREATE INDEX IF NOT EXISTS idx_status_page_subscribers_page_state
    ON status_page_subscribers(status_page_id, state);
CREATE INDEX IF NOT EXISTS idx_status_page_subscribers_confirmation_token
    ON status_page_subscribers(confirmation_token_hash);
CREATE INDEX IF NOT EXISTS idx_status_page_subscribers_manage_token
    ON status_page_subscribers(manage_token_hash);
CREATE INDEX IF NOT EXISTS idx_status_page_subscribers_unsubscribe_token
    ON status_page_subscribers(unsubscribe_token_hash);

CREATE TABLE IF NOT EXISTS status_page_subscriber_components (
    id VARCHAR(255) PRIMARY KEY,
    subscriber_id TEXT NOT NULL,
    component_id TEXT NOT NULL,
    event_scope TEXT NOT NULL DEFAULT 'all_updates',
    created_at DATETIME,
    updated_at DATETIME
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_status_page_subscriber_components_unique
    ON status_page_subscriber_components(subscriber_id, component_id, event_scope);
CREATE INDEX IF NOT EXISTS idx_status_page_subscriber_components_subscriber
    ON status_page_subscriber_components(subscriber_id);
CREATE INDEX IF NOT EXISTS idx_status_page_subscriber_components_component
    ON status_page_subscriber_components(component_id);

CREATE TABLE IF NOT EXISTS status_page_subscriber_deliveries (
    id VARCHAR(255) PRIMARY KEY,
    subscriber_id TEXT NOT NULL,
    status_page_id TEXT NOT NULL,
    public_incident_id TEXT NOT NULL DEFAULT '',
    public_incident_update_id TEXT NOT NULL DEFAULT '',
    delivery_type TEXT NOT NULL,
    delivery_state TEXT NOT NULL DEFAULT 'queued',
    provider_message_id TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    safe_error_summary TEXT NOT NULL DEFAULT '',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    queued_at DATETIME,
    sent_at DATETIME,
    failed_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_status_page_subscriber_deliveries_subscriber
    ON status_page_subscriber_deliveries(subscriber_id);
CREATE INDEX IF NOT EXISTS idx_status_page_subscriber_deliveries_page_state
    ON status_page_subscriber_deliveries(status_page_id, delivery_state);
