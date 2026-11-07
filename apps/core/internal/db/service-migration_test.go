package db

import (
	"testing"
	"time"
)

func TestServicesMigrationAddsTableAndNullableLinks(t *testing.T) {
	database := openMigrationTestDatabase(t)
	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	m := database.Migrator()
	if !m.HasTable(&Service{}) {
		t.Fatal("services table was not created")
	}
	for _, target := range []any{&App{}, &Monitor{}, &NodeAppResource{}} {
		if !m.HasColumn(target, "service_id") {
			t.Fatalf("%T has no service_id column", target)
		}
	}

	now := time.Now().UTC()
	if err := database.Create(&Service{ID: "svc_1", Name: "Checkout", Slug: "checkout", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	if err := database.Create(&Service{ID: "svc_2", Name: "Checkout again", Slug: "checkout", CreatedAt: now, UpdatedAt: now}).Error; err == nil {
		t.Fatal("duplicate slug was accepted")
	}

	// Rows written without a link keep a NULL service_id, so existing data is unaffected.
	if err := database.Create(&App{ID: "app_1", Name: "a", Slug: "a", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	var linked int64
	if err := database.Model(&App{}).Where("service_id IS NOT NULL").Count(&linked).Error; err != nil || linked != 0 {
		t.Fatalf("linked apps = %d, err = %v; want 0", linked, err)
	}
}
