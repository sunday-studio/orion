-- Rename agent to node: tables, columns, indexes and the stored values that carry the old name.
-- Old rows keep their ids; only names change. A node that was an agent keeps its id (for example agent-1234)
-- so tokens and every reference to it stay valid.

-- Tables
ALTER TABLE agent_app_resource_reports RENAME TO node_app_resource_reports;
ALTER TABLE agent_app_resources RENAME TO node_app_resources;
ALTER TABLE agent_discoveries RENAME TO node_discoveries;
ALTER TABLE agent_discovery_requests RENAME TO node_discovery_requests;
ALTER TABLE agent_monitor_definitions RENAME TO node_monitor_definitions;
ALTER TABLE agent_reports RENAME TO node_reports;
ALTER TABLE agents RENAME TO nodes;

-- Columns
ALTER TABLE node_app_resource_reports RENAME COLUMN agent_id TO node_id;
ALTER TABLE node_app_resources RENAME COLUMN agent_id TO node_id;
ALTER TABLE node_discoveries RENAME COLUMN agent_id TO node_id;
ALTER TABLE node_discovery_requests RENAME COLUMN agent_id TO node_id;
ALTER TABLE node_monitor_definitions RENAME COLUMN agent_id TO node_id;
ALTER TABLE node_reports RENAME COLUMN agent_id TO node_id;
ALTER TABLE node_reports RENAME COLUMN agent_version TO node_version;
ALTER TABLE alert_routes RENAME COLUMN agent_ids TO node_ids;
ALTER TABLE enrollment_sessions RENAME COLUMN agent_id TO node_id;
ALTER TABLE incidents RENAME COLUMN agent_id TO node_id;
ALTER TABLE monitors RENAME COLUMN agent_id TO node_id;
ALTER TABLE service_log_entries RENAME COLUMN agent_id TO node_id;

-- Indexes (SQLite cannot rename an index, so drop and create it again under the new name)
DROP INDEX IF EXISTS idx_agent_discovery_requests_agent;
CREATE INDEX idx_node_discovery_requests_node ON node_discovery_requests(node_id, created_at);
DROP INDEX IF EXISTS idx_agent_monitor_definitions_name;
CREATE UNIQUE INDEX idx_node_monitor_definitions_name ON node_monitor_definitions(node_id, name);
DROP INDEX IF EXISTS idx_agent_reports_agent_created;
CREATE INDEX idx_node_reports_node_created
  ON node_reports(node_id, created_at DESC);
DROP INDEX IF EXISTS idx_agent_reports_agent_id;
CREATE INDEX idx_node_reports_node_id ON node_reports(node_id);
DROP INDEX IF EXISTS idx_agent_reports_created_at;
CREATE INDEX idx_node_reports_created_at ON node_reports(created_at);
DROP INDEX IF EXISTS idx_agents_machine_id;
CREATE UNIQUE INDEX idx_nodes_machine_id ON nodes(machine_id);
DROP INDEX IF EXISTS idx_agents_token;
CREATE UNIQUE INDEX idx_nodes_token ON nodes(token);
DROP INDEX IF EXISTS idx_agents_token_hash;
CREATE INDEX idx_nodes_token_hash ON nodes(token_hash);
DROP INDEX IF EXISTS idx_agents_token_revoked_at;
CREATE INDEX idx_nodes_token_revoked_at ON nodes(token_revoked_at);
DROP INDEX IF EXISTS idx_incidents_agent_id;
CREATE INDEX idx_incidents_node_id ON incidents(node_id);
DROP INDEX IF EXISTS idx_monitors_agent_id;
CREATE INDEX idx_monitors_node_id ON monitors(node_id);
DROP INDEX IF EXISTS idx_monitors_agent_name_active;
CREATE UNIQUE INDEX idx_monitors_node_name_active
  ON monitors(node_id, name)
  WHERE lifecycle <> 'deleted';
DROP INDEX IF EXISTS idx_service_log_entries_agent_fingerprint;
CREATE UNIQUE INDEX idx_service_log_entries_node_fingerprint ON service_log_entries(node_id, fingerprint);
DROP INDEX IF EXISTS idx_service_log_entries_agent_time;
CREATE INDEX idx_service_log_entries_node_time ON service_log_entries(node_id, occurred_at DESC);

-- Values: Core's own system node
UPDATE nodes SET id = 'node_core' WHERE id = 'agent_core';
UPDATE node_app_resource_reports SET node_id = 'node_core' WHERE node_id = 'agent_core';
UPDATE node_app_resources SET node_id = 'node_core' WHERE node_id = 'agent_core';
UPDATE node_discoveries SET node_id = 'node_core' WHERE node_id = 'agent_core';
UPDATE node_discovery_requests SET node_id = 'node_core' WHERE node_id = 'agent_core';
UPDATE node_monitor_definitions SET node_id = 'node_core' WHERE node_id = 'agent_core';
UPDATE node_reports SET node_id = 'node_core' WHERE node_id = 'agent_core';
UPDATE enrollment_sessions SET node_id = 'node_core' WHERE node_id = 'agent_core';
UPDATE incidents SET node_id = 'node_core' WHERE node_id = 'agent_core';
UPDATE monitors SET node_id = 'node_core' WHERE node_id = 'agent_core';
UPDATE service_log_entries SET node_id = 'node_core' WHERE node_id = 'agent_core';
UPDATE audit_events SET affected_object_id = 'node_core' WHERE affected_object_type = 'agent' AND affected_object_id = 'agent_core';
UPDATE status_page_component_mappings SET resource_id = 'node_core' WHERE resource_type = 'agent' AND resource_id = 'agent_core';

-- Values: the connectivity monitor that Core raises for a silent node, and everything that points at it
CREATE TEMP TABLE _renamed_monitors AS
  SELECT id AS old_id, 'monitor-node-offline-' || node_id AS new_id FROM monitors WHERE id LIKE 'monitor-agent-offline-%';
UPDATE core_monitor_configs SET monitor_id = (SELECT new_id FROM _renamed_monitors WHERE old_id = core_monitor_configs.monitor_id) WHERE monitor_id IN (SELECT old_id FROM _renamed_monitors);
UPDATE incident_candidate_dismissals SET monitor_id = (SELECT new_id FROM _renamed_monitors WHERE old_id = incident_candidate_dismissals.monitor_id) WHERE monitor_id IN (SELECT old_id FROM _renamed_monitors);
UPDATE incidents SET monitor_id = (SELECT new_id FROM _renamed_monitors WHERE old_id = incidents.monitor_id) WHERE monitor_id IN (SELECT old_id FROM _renamed_monitors);
UPDATE log_alert_rules SET monitor_id = (SELECT new_id FROM _renamed_monitors WHERE old_id = log_alert_rules.monitor_id) WHERE monitor_id IN (SELECT old_id FROM _renamed_monitors);
UPDATE monitor_reports SET monitor_id = (SELECT new_id FROM _renamed_monitors WHERE old_id = monitor_reports.monitor_id) WHERE monitor_id IN (SELECT old_id FROM _renamed_monitors);
UPDATE monitor_uptime_rollups SET monitor_id = (SELECT new_id FROM _renamed_monitors WHERE old_id = monitor_uptime_rollups.monitor_id) WHERE monitor_id IN (SELECT old_id FROM _renamed_monitors);
UPDATE service_log_entries SET monitor_id = (SELECT new_id FROM _renamed_monitors WHERE old_id = service_log_entries.monitor_id) WHERE monitor_id IN (SELECT old_id FROM _renamed_monitors);
UPDATE status_page_component_mappings SET resource_id = (SELECT new_id FROM _renamed_monitors WHERE old_id = status_page_component_mappings.resource_id)
  WHERE resource_type = 'monitor' AND resource_id IN (SELECT old_id FROM _renamed_monitors);
UPDATE monitors SET id = (SELECT new_id FROM _renamed_monitors WHERE old_id = monitors.id) WHERE id IN (SELECT old_id FROM _renamed_monitors);
DROP TABLE _renamed_monitors;
UPDATE monitors SET type = 'node-connectivity' WHERE type = 'agent-connectivity';

-- Values: audit trail, incident timeline, logs and status page links name the kind of thing that acted or was acted on
UPDATE audit_events SET action = 'node_' || substr(action, 7) WHERE action LIKE 'agent\_%' ESCAPE '\';
UPDATE audit_events SET action = 'node.' || substr(action, 7) WHERE action LIKE 'agent.%';
UPDATE audit_events SET actor_type = 'node' WHERE actor_type = 'agent';
UPDATE audit_events SET affected_object_type = 'node' WHERE affected_object_type = 'agent';
UPDATE audit_events SET metadata_json = REPLACE(REPLACE(REPLACE(metadata_json, '"agent_', '"node_'), '"agent"', '"node"'), '"decommissioned_agents"', '"decommissioned_nodes"') WHERE metadata_json LIKE '%agent%';
UPDATE incident_events SET actor_type = 'node' WHERE actor_type = 'agent';
UPDATE service_log_entries SET source = 'node' WHERE source = 'agent';
UPDATE status_page_component_mappings SET resource_type = 'node' WHERE resource_type = 'agent';

-- Values: JSON kept in text columns
UPDATE incidents SET impacted_components = REPLACE(REPLACE(REPLACE(impacted_components, '"agent_', '"node_'), '"agent"', '"node"'), '"decommissioned_agents"', '"decommissioned_nodes"') WHERE impacted_components LIKE '%"agent%';
UPDATE incidents SET summary = REPLACE(REPLACE(REPLACE(summary, '"agent_', '"node_'), '"agent"', '"node"'), '"decommissioned_agents"', '"decommissioned_nodes"') WHERE summary LIKE '%"agent%';
UPDATE alert_groups SET summary = REPLACE(REPLACE(REPLACE(summary, '"agent_', '"node_'), '"agent"', '"node"'), '"decommissioned_agents"', '"decommissioned_nodes"') WHERE summary LIKE '%"agent%';
UPDATE log_exports SET filters_json = REPLACE(REPLACE(REPLACE(filters_json, '"agent_', '"node_'), '"agent"', '"node"'), '"decommissioned_agents"', '"decommissioned_nodes"') WHERE filters_json LIKE '%"agent%';
UPDATE log_saved_views SET filters_json = REPLACE(REPLACE(REPLACE(filters_json, '"agent_', '"node_'), '"agent"', '"node"'), '"decommissioned_agents"', '"decommissioned_nodes"') WHERE filters_json LIKE '%"agent%';
UPDATE status_page_incidents SET impact_summary = REPLACE(REPLACE(REPLACE(impact_summary, '"agent_', '"node_'), '"agent"', '"node"'), '"decommissioned_agents"', '"decommissioned_nodes"') WHERE impact_summary LIKE '%"agent%';
