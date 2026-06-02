package service

import (
	"testing"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMonitorSummaryUsesComputedHealth(t *testing.T) {
	database := setupHealthTestDB(t)
	agent := db.Agent{
		ID:        "agent-summary-computed",
		MachineId: "summary-computed-machine",
		Name:      "summary computed",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "summary-computed-token",
		LastSeen:  time.Now(),
	}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	monitor := db.Monitor{
		ID:                       "monitor-summary-computed",
		AgentID:                  agent.ID,
		Name:                     "summary computed",
		Type:                     "http",
		Lifecycle:                "active",
		Health:                   "up",
		ReportingIntervalSeconds: 60,
		CreatedAt:                time.Now(),
	}
	if err := database.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}

	now := time.Now().UTC()
	reports := []db.MonitorReport{
		{ID: "report-summary-1", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Format(time.RFC3339), Health: "up", CreatedAt: now},
		{ID: "report-summary-2", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Add(-1 * time.Minute).Format(time.RFC3339), Health: "up", CreatedAt: now.Add(-1 * time.Minute)},
		{ID: "report-summary-3", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Add(-2 * time.Minute).Format(time.RFC3339), Health: "down", CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "report-summary-4", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Add(-3 * time.Minute).Format(time.RFC3339), Health: "down", CreatedAt: now.Add(-3 * time.Minute)},
		{ID: "report-summary-5", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Add(-4 * time.Minute).Format(time.RFC3339), Health: "up", CreatedAt: now.Add(-4 * time.Minute)},
	}
	if err := database.Create(&reports).Error; err != nil {
		t.Fatalf("create reports: %v", err)
	}

	service := NewMonitorService(database, utils.NewLogger())
	summary, err := service.GetMonitorSummary()
	if err != nil {
		t.Fatalf("GetMonitorSummary() error = %v", err)
	}
	if summary.Total != 1 || summary.Degraded != 1 || summary.Up != 0 || summary.Unknown != 0 {
		t.Fatalf("summary = %+v, want one computed degraded monitor", summary)
	}
}

func setupHealthTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	return database
}
