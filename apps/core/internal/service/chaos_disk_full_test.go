package service

import (
	"fmt"
	"orion/core/internal/config"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"
)

// Chaos 7: the main database fills up. Writes must fail with an error (the API turns this into
// a 5xx the agent retries), nothing may half-commit or corrupt, and the same calls succeed once
// space returns. PRAGMA max_page_count reproduces SQLITE_FULL without root or a loop device.
func TestChaosMainDatabaseDiskFullFailsCleanlyAndRecovers(t *testing.T) {
	database := testkit.NewDB(t)
	sqlDB, _ := database.DB()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)
	a, _ := testkit.TwoAgents(t, database, time.Now().UTC())
	reports := NewReportService(database, utils.NewLogger(), &config.Config{})
	payload := func(i int) MonitorReportPayload {
		return MonitorReportPayload{Timestamp: time.Now().UTC().Format(time.RFC3339), Health: "up", Metrics: map[string]any{"blob": strings.Repeat("x", 2000), "i": i}}
	}
	for i := 0; i < 50; i++ {
		if _, err := reports.StoreMonitorReport(a.Monitor.ID, payload(i)); err != nil {
			t.Fatal(err)
		}
	}
	var pages int
	database.Raw("PRAGMA page_count").Scan(&pages)
	database.Exec(fmt.Sprintf("PRAGMA max_page_count = %d", pages))

	var failed int
	for i := 0; i < 400; i++ {
		if _, err := reports.StoreMonitorReport(a.Monitor.ID, payload(1000+i)); err != nil {
			if !strings.Contains(err.Error(), "database or disk is full") {
				t.Fatalf("unexpected error kind: %v", err)
			}
			failed++
		}
	}
	if failed == 0 {
		t.Fatal("no write failed with the page cap in place; the test is not exercising SQLITE_FULL")
	}
	var check string
	database.Raw("PRAGMA integrity_check").Scan(&check)
	if check != "ok" {
		t.Fatalf("integrity_check = %q after disk-full writes", check)
	}
	// Reads and the health computation still work while writes fail.
	if _, err := NewHealthService(database, utils.NewLogger()).ComputeMonitorHealth(a.Monitor.ID, DefaultHealthConfig()); err != nil {
		t.Fatalf("health read failed while disk full: %v", err)
	}

	database.Exec("PRAGMA max_page_count = 1073741823")
	if _, err := reports.StoreMonitorReport(a.Monitor.ID, payload(9999)); err != nil {
		t.Fatalf("write after space returned: %v", err)
	}
}
