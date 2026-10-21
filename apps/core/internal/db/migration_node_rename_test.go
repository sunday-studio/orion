package db

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"testing"
)

func TestRenameMigrationKeepsDataAndRewritesStoredNames(t *testing.T) {
	database := openMigrationTestDatabase(t)
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at DATETIME NOT NULL, checksum TEXT NOT NULL DEFAULT '')`); err != nil {
		t.Fatal(err)
	}
	migrations, err := loadMigrations("migrations")
	if err != nil {
		t.Fatal(err)
	}
	migrations = append(migrations, goMigrations()...)
	slices.SortFunc(migrations, func(a, b migration) int { return cmp.Compare(a.version, b.version) })
	for _, m := range migrations {
		if m.version <= 54 {
			if _, err := applyMigration(context.Background(), sqlDB, m); err != nil {
				t.Fatalf("apply %s: %v", m.name, err)
			}
		}
	}

	// What an installation had under the old names.
	for _, stmt := range []string{
		`INSERT INTO agents (id, machine_id, name, os, arch, token, token_hash, token_version, token_revocation_reason, system, labels, notes, previous_token_hash) VALUES ('agent-1', 'm1', 'edge-1', 'linux', 'amd64', 't1', 'h1', 1, '', 0, '', '', '')`,
		`INSERT INTO agents (id, machine_id, name, os, arch, token, token_hash, token_version, token_revocation_reason, system, labels, notes, previous_token_hash) VALUES ('agent_core', 'core', 'Core', 'core', 'core', 't0', 'h0', 1, '', 1, '', '', '')`,
		`INSERT INTO agent_reports (id, agent_id, agent_version, uptime_seconds) VALUES ('r1', 'agent-1', '0.9.3', 10)`,
		`INSERT INTO agent_discoveries (agent_id, enabled, kinds, items, capabilities, received_at) VALUES ('agent-1', 1, '[]', '[]', '["requests"]', CURRENT_TIMESTAMP)`,
		`INSERT INTO monitors (id, type, name, agent_id, lifecycle, health, active_incident_id, incident_state, system, runbook_url, maintenance_reason) VALUES ('monitor-agent-offline-agent-1', 'agent-connectivity', 'Agent connectivity', 'agent-1', 'active', 'up', '', '', 1, '', '')`,
		`INSERT INTO monitors (id, type, name, agent_id, lifecycle, health, active_incident_id, incident_state, system, runbook_url, maintenance_reason) VALUES ('m-http', 'http', 'site', 'agent-1', 'active', 'up', '', '', 0, '', '')`,
		`INSERT INTO monitor_reports (id, monitor_id, payload, collected_at, health, client_report_id) VALUES ('mr1', 'monitor-agent-offline-agent-1', '{}', 'x', 'up', '')`,
		`INSERT INTO incidents (id, status, severity, title, agent_id, monitor_id, opened_at, last_event_at, notification_status, impacted_components, resolution_kind, number, responder, summary, source, covered_by_incident_id) VALUES ('i1', 'open', 'high', 't', 'agent-1', 'monitor-agent-offline-agent-1', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'pending', '[{"type":"agent","id":"agent-1"}]', '', 1, '', '{"agent_id":"agent-1"}', 'agent', '')`,
		`INSERT INTO incident_events (id, incident_id, type, actor_type, actor_id, note) VALUES ('e1', 'i1', 'opened', 'agent', 'agent-1', '')`,
		`INSERT INTO audit_events (id, action, status_page_id, affected_object_type, affected_object_id, actor_type, actor_id, created_at, metadata_json) VALUES ('a1', 'agent_token_rotated', '', 'agent', 'agent-1', 'agent', 'agent-1', CURRENT_TIMESTAMP, '{"agent_name":"edge-1","agent_reports_archived":3,"note":"agent restarted"}')`,
		`INSERT INTO audit_events (id, action, status_page_id, affected_object_type, affected_object_id, actor_type, actor_id, created_at, metadata_json) VALUES ('a2', 'agent.maintenance_enabled', '', 'agent', 'agent-1', 'admin', 'admin', CURRENT_TIMESTAMP, '{}')`,
		`INSERT INTO audit_events (id, action, status_page_id, affected_object_type, affected_object_id, actor_type, actor_id, created_at, metadata_json) VALUES ('a3', 'server_edited', '', 'agent', 'agent_core', 'admin', 'admin', CURRENT_TIMESTAMP, '{}')`,
		`INSERT INTO service_log_entries (id, agent_id, monitor_id, source, stream, level, component, monitor_name, message, fields_json, raw, fingerprint, occurred_at, collected_at) VALUES ('l1', 'agent-1', '', 'agent', 'jsonl', 'INFO', '', '', 'm', '{}', '', 'f1', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO status_page_component_mappings (id, component_id, resource_type, resource_id, health_rollup_strategy, uptime_rollup_strategy) VALUES ('sm1', 'c', 'agent', 'agent-1', 'worst', 'avg')`,
		`INSERT INTO status_page_component_mappings (id, component_id, resource_type, resource_id, health_rollup_strategy, uptime_rollup_strategy) VALUES ('sm2', 'c', 'monitor', 'monitor-agent-offline-agent-1', 'worst', 'avg')`,
		`INSERT INTO alert_routes (id, name, agent_ids) VALUES ('ar1', 'route', '["agent-1"]')`,
		`INSERT INTO alert_groups (id, group_key, status, event_type, severity, summary, first_incident_id, last_incident_id, incident_count, first_event_at, last_event_at) VALUES ('g1', 'agent:agent-1|monitor_type:http|severity:high', 'open', 'incident_opened', 'high', '{"agent_id":"agent-1"}', 'i1', 'i1', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
	} {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	one := func(q string, args ...any) string {
		var out string
		if err := sqlDB.QueryRow(q, args...).Scan(&out); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return out
	}
	// Tables and columns moved, rows stayed, ids (and with them tokens) were kept.
	if got := one(`SELECT name || '/' || token FROM nodes WHERE id = 'agent-1'`); got != "edge-1/t1" {
		t.Fatalf("node row = %q", got)
	}
	if got := one(`SELECT node_version FROM node_reports WHERE node_id = 'agent-1'`); got != "0.9.3" {
		t.Fatalf("report version = %q", got)
	}
	if got := one(`SELECT capabilities FROM node_discoveries WHERE node_id = 'agent-1'`); got != `["requests"]` {
		t.Fatalf("discovery = %q", got)
	}
	if got := one(`SELECT node_ids FROM alert_routes WHERE id = 'ar1'`); got != `["agent-1"]` {
		t.Fatalf("alert route = %q", got)
	}
	// The system node, the silent-node monitor and everything pointing at it follow the new names.
	if got := one(`SELECT id FROM nodes WHERE system = 1`); got != "node_core" {
		t.Fatalf("system node id = %q", got)
	}
	if got := one(`SELECT id || '/' || type || '/' || node_id FROM monitors WHERE system = 1`); got != "monitor-node-offline-agent-1/node-connectivity/agent-1" {
		t.Fatalf("connectivity monitor = %q", got)
	}
	for _, q := range []string{
		`SELECT monitor_id FROM monitor_reports WHERE id = 'mr1'`,
		`SELECT monitor_id FROM incidents WHERE id = 'i1'`,
		`SELECT resource_id FROM status_page_component_mappings WHERE id = 'sm2'`,
	} {
		if got := one(q); got != "monitor-node-offline-agent-1" {
			t.Fatalf("%s = %q, want the renamed monitor id", q, got)
		}
	}
	// Stored kinds and JSON keys.
	for q, want := range map[string]string{
		`SELECT action FROM audit_events WHERE id = 'a1'`:                                    "node_token_rotated",
		`SELECT action FROM audit_events WHERE id = 'a2'`:                                    "node.maintenance_enabled",
		`SELECT actor_type || '/' || affected_object_type FROM audit_events WHERE id = 'a1'`: "node/node",
		`SELECT affected_object_id FROM audit_events WHERE id = 'a3'`:                        "node_core",
		`SELECT actor_type FROM incident_events WHERE id = 'e1'`:                             "node",
		`SELECT source FROM service_log_entries WHERE id = 'l1'`:                             "node",
		`SELECT resource_type FROM status_page_component_mappings WHERE id = 'sm1'`:          "node",
		`SELECT group_key FROM alert_groups WHERE id = 'g1'`:                                 "node:agent-1|monitor_type:http|severity:high",
		`SELECT summary FROM alert_groups WHERE id = 'g1'`:                                   `{"node_id":"agent-1"}`,
		`SELECT impacted_components FROM incidents WHERE id = 'i1'`:                          `[{"type":"node","id":"agent-1"}]`,
		`SELECT summary FROM incidents WHERE id = 'i1'`:                                      `{"node_id":"agent-1"}`,
	} {
		if got := one(q); got != want {
			t.Errorf("%s = %q, want %q", q, got, want)
		}
	}
	// Free text is history and stays as written; only keys and kinds change.
	if got := one(`SELECT metadata_json FROM audit_events WHERE id = 'a1'`); !strings.Contains(got, `"node_name":"edge-1"`) || !strings.Contains(got, `"node_reports_archived":3`) || !strings.Contains(got, "agent restarted") {
		t.Errorf("audit metadata = %q", got)
	}
	// Nothing is left under the old schema names.
	var left string
	if err := sqlDB.QueryRow(`SELECT COALESCE(group_concat(name), '') FROM sqlite_master WHERE sql LIKE '%agent%' AND name NOT IN ('service_log_entries')`).Scan(&left); err != nil || left != "" {
		t.Fatalf("schema objects still naming agent: %q (%v)", left, err)
	}
}
