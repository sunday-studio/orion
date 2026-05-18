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
	if !database.Migrator().HasTable(&AlertChannel{}) {
		t.Fatal("alert_channels table was not created")
	}

	var count int64
	if err := database.Table("schema_migrations").Where("version = ?", 1).Count(&count).Error; err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if count != 1 {
		t.Fatalf("migration version count = %d, want 1", count)
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
	if count != 5 {
		t.Fatalf("migration count = %d, want 5", count)
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
}

func openMigrationTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return database
}
