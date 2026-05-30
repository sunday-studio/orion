package db

import "testing"

func TestAlertLifecycleMigrationDefaultsSupportRawSQLInserts(t *testing.T) {
	database := openMigrationTestDatabase(t)
	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	if _, err := sqlDB.Exec(`
		INSERT INTO alert_deliveries (id, incident_id, event_type, channel, type, status)
		VALUES ('delivery-defaults', 'incident-defaults', 'incident_opened', 'ops', 'webhook', 'pending');
	`); err != nil {
		t.Fatalf("insert minimal alert delivery: %v", err)
	}

	var delivery AlertDelivery
	if err := database.Where("id = ?", "delivery-defaults").First(&delivery).Error; err != nil {
		t.Fatalf("find delivery: %v", err)
	}
	if delivery.AttemptCount != 0 || delivery.MaxAttempts != 3 || delivery.RouteID != "" || delivery.AlertGroupID != "" {
		t.Fatalf("delivery defaults = %+v, want retry and route/group defaults", delivery)
	}

	if _, err := sqlDB.Exec(`
		INSERT INTO alert_routes (id, name, enabled, priority)
		VALUES ('route-defaults', 'route defaults', 1, 100);
	`); err != nil {
		t.Fatalf("insert minimal alert route: %v", err)
	}

	var route AlertRoute
	if err := database.Where("id = ?", "route-defaults").First(&route).Error; err != nil {
		t.Fatalf("find route: %v", err)
	}
	if route.GroupingPolicy != AlertGroupingPolicySuppress || route.GroupingDelaySeconds != DefaultAlertGroupingDelaySeconds {
		t.Fatalf("route defaults = %+v, want suppress grouping defaults", route)
	}
}
