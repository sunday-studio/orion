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
	if count != 4 {
		t.Fatalf("migration count = %d, want 4", count)
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
