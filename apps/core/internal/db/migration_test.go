package db

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrateAppliesEmbeddedMigrations(t *testing.T) {
	database := openMigrationTestDatabase(t)

	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	if !database.Migrator().HasTable(&Agent{}) {
		t.Fatal("agents table was not created")
	}
	if !database.Migrator().HasTable(&AlertDelivery{}) {
		t.Fatal("alert_deliveries table was not created")
	}
	if !database.Migrator().HasTable(&AlertDeliveryAttempt{}) {
		t.Fatal("alert_delivery_attempts table was not created")
	}
	if !database.Migrator().HasTable(&AlertChannel{}) {
		t.Fatal("alert_channels table was not created")
	}
	if !database.Migrator().HasColumn(&AlertChannel{}, "webhook_signing_secret") {
		t.Fatal("alert_channels.webhook_signing_secret was not created")
	}
	if !database.Migrator().HasTable(&AlertRoute{}) {
		t.Fatal("alert_routes table was not created")
	}
	if !database.Migrator().HasColumn(&AlertDelivery{}, "route_id") {
		t.Fatal("alert_deliveries.route_id was not created")
	}
	if !database.Migrator().HasColumn(&AlertRoute{}, "grouping_policy") {
		t.Fatal("alert_routes.grouping_policy was not created")
	}
	if !database.Migrator().HasColumn(&AlertRoute{}, "grouping_delay_seconds") {
		t.Fatal("alert_routes.grouping_delay_seconds was not created")
	}
	if !database.Migrator().HasTable(&AlertGroup{}) {
		t.Fatal("alert_groups table was not created")
	}
	if !database.Migrator().HasTable(&AlertGroupMember{}) {
		t.Fatal("alert_group_members table was not created")
	}
	if !database.Migrator().HasColumn(&AlertDelivery{}, "alert_group_id") {
		t.Fatal("alert_deliveries.alert_group_id was not created")
	}
	if !database.Migrator().HasTable(&StatusPage{}) {
		t.Fatal("status_pages table was not created")
	}
	if !database.Migrator().HasColumn(&StatusPage{}, "custom_domain") {
		t.Fatal("status_pages.custom_domain was not created")
	}
	if !database.Migrator().HasTable(&StatusPageSection{}) {
		t.Fatal("status_page_sections table was not created")
	}
	if !database.Migrator().HasTable(&StatusPageComponent{}) {
		t.Fatal("status_page_components table was not created")
	}
	if !database.Migrator().HasTable(&StatusPageComponentMapping{}) {
		t.Fatal("status_page_component_mappings table was not created")
	}
	if !database.Migrator().HasTable(&StatusPageIncident{}) {
		t.Fatal("status_page_incidents table was not created")
	}
	if !database.Migrator().HasTable(&StatusPageIncidentUpdate{}) {
		t.Fatal("status_page_incident_updates table was not created")
	}
	if !database.Migrator().HasColumn(&Incident{}, "impacted_components") {
		t.Fatal("incidents.impacted_components was not created")
	}
	for _, column := range []string{"covered_at", "covered_until", "coverage_note", "resolution_kind", "reopened_at", "reopen_count"} {
		if !database.Migrator().HasColumn(&Incident{}, column) {
			t.Fatalf("incidents.%s was not created", column)
		}
	}
	if !database.Migrator().HasTable(&AuditEvent{}) {
		t.Fatal("audit_events table was not created")
	}
	if !database.Migrator().HasColumn(&AuditEvent{}, "metadata_json") {
		t.Fatal("audit_events.metadata_json was not created")
	}
	for _, column := range []string{"token_hash", "token_version", "token_rotated_at", "token_revoked_at", "token_revocation_reason"} {
		if !database.Migrator().HasColumn(&Agent{}, column) {
			t.Fatalf("agents.%s was not created", column)
		}
	}
	if !database.Migrator().HasTable(&StatusPageSubscriber{}) {
		t.Fatal("status_page_subscribers table was not created")
	}
	if !database.Migrator().HasTable(&StatusPageSubscriberComponent{}) {
		t.Fatal("status_page_subscriber_components table was not created")
	}
	if !database.Migrator().HasTable(&StatusPageSubscriberDelivery{}) {
		t.Fatal("status_page_subscriber_deliveries table was not created")
	}
	if !database.Migrator().HasTable(&CoreMonitorConfig{}) {
		t.Fatal("core_monitor_configs table was not created")
	}
	if !database.Migrator().HasColumn(&CoreMonitorConfig{}, "heartbeat_token_hash") {
		t.Fatal("core_monitor_configs.heartbeat_token_hash was not created")
	}
	if !database.Migrator().HasColumn(&CoreMonitorConfig{}, "last_signal_at") {
		t.Fatal("core_monitor_configs.last_signal_at was not created")
	}
	if !database.Migrator().HasTable(&CoreWorkerStatus{}) {
		t.Fatal("core_worker_statuses table was not created")
	}
	if !database.Migrator().HasTable(&ServiceLogEntry{}) {
		t.Fatal("service_log_entries table was not created")
	}

	var count int64
	if err := database.Table("schema_migrations").Where("version = ?", 1).Count(&count).Error; err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if count != 1 {
		t.Fatalf("migration version count = %d, want 1", count)
	}
}

func TestEmbeddedMigrationVersionsAreContiguous(t *testing.T) {
	migrations, err := loadMigrations("migrations")
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	for index, migration := range migrations {
		want := index + 1
		if migration.version != want {
			t.Fatalf("migration %s has version %d, want %d", migration.name, migration.version, want)
		}
	}
}

func TestMigrateAppliesStatusPageSchema(t *testing.T) {
	database := openMigrationTestDatabase(t)

	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	for _, table := range []struct {
		name  string
		model any
	}{
		{name: "status_pages", model: &StatusPage{}},
		{name: "status_page_sections", model: &StatusPageSection{}},
		{name: "status_page_components", model: &StatusPageComponent{}},
		{name: "status_page_component_mappings", model: &StatusPageComponentMapping{}},
		{name: "status_page_incidents", model: &StatusPageIncident{}},
		{name: "status_page_incident_updates", model: &StatusPageIncidentUpdate{}},
		{name: "audit_events", model: &AuditEvent{}},
		{name: "status_page_subscribers", model: &StatusPageSubscriber{}},
		{name: "status_page_subscriber_components", model: &StatusPageSubscriberComponent{}},
		{name: "status_page_subscriber_deliveries", model: &StatusPageSubscriberDelivery{}},
	} {
		if !database.Migrator().HasTable(table.model) {
			t.Fatalf("%s table was not created", table.name)
		}
	}

	for _, column := range []string{
		"custom_domain",
		"seo_title",
		"seo_description",
		"open_graph_image_url",
		"canonical_url",
		"default_incident_visibility",
	} {
		if !database.Migrator().HasColumn(&StatusPage{}, column) {
			t.Fatalf("status_pages.%s was not created", column)
		}
	}

	for _, column := range []string{
		"destination_value_ciphertext",
		"confirmation_token_hash",
		"manage_token_hash",
		"unsubscribe_token_hash",
		"bounce_count",
		"last_delivery_status",
	} {
		if !database.Migrator().HasColumn(&StatusPageSubscriber{}, column) {
			t.Fatalf("status_page_subscribers.%s was not created", column)
		}
	}

	for _, index := range []struct {
		model any
		name  string
	}{
		{model: &StatusPage{}, name: "idx_status_pages_slug"},
		{model: &StatusPage{}, name: "idx_status_pages_custom_domain"},
		{model: &StatusPageComponent{}, name: "idx_status_page_components_page_visible_sort"},
		{model: &StatusPageComponentMapping{}, name: "idx_status_page_component_mappings_resource"},
		{model: &StatusPageIncident{}, name: "idx_status_page_incidents_page_visibility"},
		{model: &StatusPageIncidentUpdate{}, name: "idx_status_page_incident_updates_incident_published"},
		{model: &AuditEvent{}, name: "idx_audit_events_affected_object"},
		{model: &StatusPageSubscriber{}, name: "idx_status_page_subscribers_destination"},
		{model: &StatusPageSubscriberComponent{}, name: "idx_status_page_subscriber_components_unique"},
		{model: &StatusPageSubscriberDelivery{}, name: "idx_status_page_subscriber_deliveries_page_state"},
	} {
		if !database.Migrator().HasIndex(index.model, index.name) {
			t.Fatalf("%s index was not created", index.name)
		}
	}
}

func TestStatusPageUniqueIndexesMatchSeedAssumptions(t *testing.T) {
	database := openMigrationTestDatabase(t)

	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get database handle: %v", err)
	}
	if _, err := sqlDB.Exec(`
		INSERT INTO status_pages (id, slug, custom_domain, title) VALUES
			('page-empty-domain-a', 'empty-domain-a', '', 'Empty domain A'),
			('page-empty-domain-b', 'empty-domain-b', '', 'Empty domain B'),
			('page-custom-domain-a', 'custom-domain-a', 'status.example.test', 'Custom domain A');
	`); err != nil {
		t.Fatalf("insert status pages: %v", err)
	}
	if _, err := sqlDB.Exec(`
		INSERT INTO status_pages (id, slug, custom_domain, title)
		VALUES ('page-custom-domain-b', 'custom-domain-b', 'status.example.test', 'Custom domain B');
	`); err == nil {
		t.Fatal("duplicate non-empty custom domain insert succeeded")
	}

	if _, err := sqlDB.Exec(`
		INSERT INTO status_page_subscribers (id, status_page_id, destination_type, destination_hash, masked_destination) VALUES
			('subscriber-a', 'page-empty-domain-a', 'email', 'hash-a', 'a@example.test'),
			('subscriber-b', 'page-empty-domain-a', 'email', 'hash-b', 'b@example.test');
	`); err != nil {
		t.Fatalf("insert status page subscribers: %v", err)
	}
	if _, err := sqlDB.Exec(`
		INSERT INTO status_page_subscribers (id, status_page_id, destination_type, destination_hash, masked_destination)
		VALUES ('subscriber-c', 'page-empty-domain-a', 'email', 'hash-a', 'c@example.test');
	`); err == nil {
		t.Fatal("duplicate status page subscriber destination insert succeeded")
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	database := openMigrationTestDatabase(t)

	if err := Migrate(database); err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}
	if err := Migrate(database); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var count int64
	if err := database.Table("schema_migrations").Count(&count).Error; err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	migrations, err := loadMigrations("migrations")
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	uniqueVersions := map[int]bool{}
	for _, migration := range migrations {
		uniqueVersions[migration.version] = true
	}
	if count != int64(len(uniqueVersions)) {
		t.Fatalf("migration count = %d, want %d", count, len(uniqueVersions))
	}
}

func TestMigrateRepairsLegacyAgentReportMetadataColumns(t *testing.T) {
	database := openMigrationTestDatabase(t)
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get database handle: %v", err)
	}
	if _, err := sqlDB.Exec(`
		CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at DATETIME NOT NULL);
		INSERT INTO schema_migrations (version, name, applied_at) VALUES
			(1, '000001_init_schema.up.sql', CURRENT_TIMESTAMP),
			(2, '000002_data_lifecycle_settings.up.sql', CURRENT_TIMESTAMP),
			(3, '000003_monitor_uptime_rollups.up.sql', CURRENT_TIMESTAMP),
			(4, '000004_incident_reconciliation_state.up.sql', CURRENT_TIMESTAMP);
		CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			machine_id TEXT NOT NULL,
			name TEXT NOT NULL,
			os TEXT NOT NULL,
			arch TEXT NOT NULL,
			token TEXT NOT NULL,
			reporting_interval_seconds INTEGER DEFAULT 60,
			created_at DATETIME,
			last_seen DATETIME
		);
		CREATE TABLE agent_reports (
			id VARCHAR(255) PRIMARY KEY,
			agent_id TEXT NOT NULL,
			created_at DATETIME,
			uptime_seconds INTEGER,
			timestamp TEXT,
			cpu JSON,
			memory JSON,
			disk JSON,
			location JSON
		);
		CREATE TABLE alert_deliveries (
			id VARCHAR(255) PRIMARY KEY,
			incident_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			channel TEXT NOT NULL,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			error TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
		CREATE TABLE incidents (
			id VARCHAR(255) PRIMARY KEY,
			status TEXT NOT NULL,
			severity TEXT NOT NULL,
			title TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			monitor_id TEXT NOT NULL,
			opened_at DATETIME NOT NULL,
			resolved_at DATETIME,
			last_event_at DATETIME NOT NULL,
			latest_event TEXT,
			notification_status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME,
			updated_at DATETIME
		);
		INSERT INTO incidents (
			id,
			status,
			severity,
			title,
			agent_id,
			monitor_id,
			opened_at,
			last_event_at,
			notification_status
		) VALUES (
			'incident-legacy',
			'open',
			'high',
			'legacy incident',
			'agent-legacy',
			'monitor-legacy',
			CURRENT_TIMESTAMP,
			CURRENT_TIMESTAMP,
			'pending'
		);
	`); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}

	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	for _, column := range []string{"agent_version", "config_summary"} {
		if !database.Migrator().HasColumn(&AgentReport{}, column) {
			t.Fatalf("agent_reports.%s was not added", column)
		}
	}
	if !database.Migrator().HasColumn(&Incident{}, "impacted_components") {
		t.Fatal("incidents.impacted_components was not added")
	}
	var impactedComponents string
	if err := database.Table("incidents").Select("impacted_components").Where("id = ?", "incident-legacy").Scan(&impactedComponents).Error; err != nil {
		t.Fatalf("read legacy incident impacted components: %v", err)
	}
	if impactedComponents != "[]" {
		t.Fatalf("legacy incident impacted_components = %q, want []", impactedComponents)
	}
	for _, column := range []string{"covered_at", "covered_until", "coverage_note", "resolution_kind", "reopened_at", "reopen_count"} {
		if !database.Migrator().HasColumn(&Incident{}, column) {
			t.Fatalf("incidents.%s was not added", column)
		}
	}
	var lifecycleDefaults struct {
		ResolutionKind string
		ReopenCount    int
	}
	if err := database.Table("incidents").Select("resolution_kind, reopen_count").Where("id = ?", "incident-legacy").Scan(&lifecycleDefaults).Error; err != nil {
		t.Fatalf("read legacy incident lifecycle defaults: %v", err)
	}
	if lifecycleDefaults.ResolutionKind != "" || lifecycleDefaults.ReopenCount != 0 {
		t.Fatalf("legacy lifecycle defaults = %+v, want empty resolution kind and zero reopen count", lifecycleDefaults)
	}
}

func openMigrationTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return database
}
