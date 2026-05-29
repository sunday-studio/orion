CREATE TABLE IF NOT EXISTS status_pages (
    id VARCHAR(255) PRIMARY KEY,
    slug TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    seo_title TEXT,
    seo_description TEXT,
    open_graph_image_url TEXT,
    canonical_url TEXT,
    visibility TEXT NOT NULL DEFAULT 'draft',
    theme_settings TEXT NOT NULL DEFAULT '{}',
    default_incident_visibility TEXT NOT NULL DEFAULT 'draft',
    published_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_status_pages_slug ON status_pages(slug);
CREATE INDEX IF NOT EXISTS idx_status_pages_visibility ON status_pages(visibility);
CREATE INDEX IF NOT EXISTS idx_status_pages_published_at ON status_pages(published_at);

CREATE TABLE IF NOT EXISTS status_page_sections (
    id VARCHAR(255) PRIMARY KEY,
    status_page_id TEXT NOT NULL,
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    collapsed_by_default BOOLEAN NOT NULL DEFAULT false,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_status_page_sections_page_sort
    ON status_page_sections(status_page_id, sort_order);

CREATE TABLE IF NOT EXISTS status_page_components (
    id VARCHAR(255) PRIMARY KEY,
    status_page_id TEXT NOT NULL,
    section_id TEXT NOT NULL,
    public_name TEXT NOT NULL,
    public_description TEXT,
    display_mode TEXT NOT NULL DEFAULT 'single_resource',
    manual_status TEXT,
    manual_status_reason TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    visible BOOLEAN NOT NULL DEFAULT true,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_status_page_components_page_visible_sort
    ON status_page_components(status_page_id, visible, sort_order);
CREATE INDEX IF NOT EXISTS idx_status_page_components_section_sort
    ON status_page_components(section_id, sort_order);

CREATE TABLE IF NOT EXISTS status_page_component_mappings (
    id VARCHAR(255) PRIMARY KEY,
    component_id TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    health_rollup_strategy TEXT NOT NULL DEFAULT 'worst',
    uptime_rollup_strategy TEXT NOT NULL DEFAULT 'worst',
    created_at DATETIME,
    updated_at DATETIME,
    UNIQUE(component_id, resource_type, resource_id)
);

CREATE INDEX IF NOT EXISTS idx_status_page_component_mappings_component_id
    ON status_page_component_mappings(component_id);
CREATE INDEX IF NOT EXISTS idx_status_page_component_mappings_resource
    ON status_page_component_mappings(resource_type, resource_id);

CREATE TABLE IF NOT EXISTS status_page_incidents (
    id VARCHAR(255) PRIMARY KEY,
    status_page_id TEXT NOT NULL,
    internal_incident_id TEXT,
    title TEXT NOT NULL,
    public_status TEXT NOT NULL,
    severity TEXT NOT NULL,
    impact_summary TEXT,
    visibility TEXT NOT NULL DEFAULT 'draft',
    affected_component_ids TEXT NOT NULL DEFAULT '[]',
    published_at DATETIME,
    resolved_at DATETIME,
    scheduled_start_at DATETIME,
    scheduled_end_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_status_page_incidents_page_visibility
    ON status_page_incidents(status_page_id, visibility);
CREATE INDEX IF NOT EXISTS idx_status_page_incidents_internal_incident_id
    ON status_page_incidents(internal_incident_id);
CREATE INDEX IF NOT EXISTS idx_status_page_incidents_public_status
    ON status_page_incidents(public_status);
CREATE INDEX IF NOT EXISTS idx_status_page_incidents_published_at
    ON status_page_incidents(published_at);

CREATE TABLE IF NOT EXISTS status_page_incident_updates (
    id VARCHAR(255) PRIMARY KEY,
    incident_id TEXT NOT NULL,
    status TEXT NOT NULL,
    message TEXT NOT NULL,
    created_by TEXT,
    published_at DATETIME,
    created_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_status_page_incident_updates_incident_published
    ON status_page_incident_updates(incident_id, published_at);
