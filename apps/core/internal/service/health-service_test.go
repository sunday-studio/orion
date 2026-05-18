package service

import (
	"testing"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/logging"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestComputeAgentHealthWithoutMonitors(t *testing.T) {
	tests := []struct {
		name   string
		agent  db.Agent
		expect string
	}{
		{
			name: "fresh agent is up without monitors",
			agent: db.Agent{
				ID:        "agent-fresh",
				MachineId: "fresh-machine",
				Name:      "fresh",
				OS:        "linux",
				Arch:      "arm64",
				Token:     "fresh-token",
				LastSeen:  time.Now(),
			},
			expect: "up",
		},
		{
			name: "stale agent is stale without monitors",
			agent: db.Agent{
				ID:        "agent-stale",
				MachineId: "stale-machine",
				Name:      "stale",
				OS:        "linux",
				Arch:      "arm64",
				Token:     "stale-token",
				LastSeen:  time.Now().Add(-30 * time.Minute),
			},
			expect: "stale",
		},
		{
			name: "agent stale window follows reporting interval",
			agent: db.Agent{
				ID:                       "agent-slow",
				MachineId:                "slow-machine",
				Name:                     "slow",
				OS:                       "linux",
				Arch:                     "arm64",
				Token:                    "slow-token",
				LastSeen:                 time.Now().Add(-20 * time.Minute),
				ReportingIntervalSeconds: 600,
			},
			expect: "up",
		},
		{
			name: "maintenance agent reports maintenance",
			agent: db.Agent{
				ID:              "agent-maintenance",
				MachineId:       "maintenance-machine",
				Name:            "maintenance",
				OS:              "linux",
				Arch:            "arm64",
				Token:           "maintenance-token",
				LastSeen:        time.Now().Add(-30 * time.Minute),
				MaintenanceMode: true,
			},
			expect: "maintenance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := setupHealthTestDB(t)
			if err := database.Create(&tt.agent).Error; err != nil {
				t.Fatalf("create agent: %v", err)
			}

			service := NewHealthService(database, logging.NewLogger())
			got, upCount, downCount, degradedCount, err := service.ComputeAgentHealth(tt.agent.ID, DefaultHealthConfig())
			if err != nil {
				t.Fatalf("ComputeAgentHealth() error = %v", err)
			}

			if got != tt.expect {
				t.Fatalf("ComputeAgentHealth() = %q, want %q", got, tt.expect)
			}
			if upCount != 0 || downCount != 0 || degradedCount != 0 {
				t.Fatalf("counts = up:%d down:%d degraded:%d, want all zero", upCount, downCount, degradedCount)
			}
		})
	}
}

func TestComputeAgentHealthCountsStoredMonitorHealthForStaleAgent(t *testing.T) {
	database := setupHealthTestDB(t)
	agent := db.Agent{
		ID:        "agent-stale-with-monitors",
		MachineId: "stale-with-monitors-machine",
		Name:      "stale with monitors",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "stale-with-monitors-token",
		LastSeen:  time.Now().Add(-30 * time.Minute),
	}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	monitors := []db.Monitor{
		{
			ID:        "monitor-up",
			AgentID:   agent.ID,
			Name:      "up",
			Type:      "http",
			Lifecycle: "active",
			Health:    "up",
		},
		{
			ID:        "monitor-down",
			AgentID:   agent.ID,
			Name:      "down",
			Type:      "http",
			Lifecycle: "active",
			Health:    "down",
		},
		{
			ID:        "monitor-degraded",
			AgentID:   agent.ID,
			Name:      "degraded",
			Type:      "http",
			Lifecycle: "active",
			Health:    "degraded",
		},
	}
	if err := database.Create(&monitors).Error; err != nil {
		t.Fatalf("create monitors: %v", err)
	}

	service := NewHealthService(database, logging.NewLogger())
	got, upCount, downCount, degradedCount, err := service.ComputeAgentHealth(agent.ID, DefaultHealthConfig())
	if err != nil {
		t.Fatalf("ComputeAgentHealth() error = %v", err)
	}

	if got != "stale" {
		t.Fatalf("ComputeAgentHealth() = %q, want stale", got)
	}
	if upCount != 1 || downCount != 1 || degradedCount != 1 {
		t.Fatalf("counts = up:%d down:%d degraded:%d, want 1/1/1", upCount, downCount, degradedCount)
	}
}

func TestDetectStaleMonitorsUsesReportingInterval(t *testing.T) {
	database := setupHealthTestDB(t)
	agent := db.Agent{
		ID:        "agent-monitor-stale-window",
		MachineId: "monitor-stale-window-machine",
		Name:      "monitor stale window",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "monitor-stale-window-token",
		LastSeen:  time.Now(),
	}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	monitors := []db.Monitor{
		{
			ID:                       "monitor-fast",
			AgentID:                  agent.ID,
			Name:                     "fast",
			Type:                     "http",
			Lifecycle:                "active",
			Health:                   "up",
			ReportingIntervalSeconds: 60,
		},
		{
			ID:                       "monitor-slow",
			AgentID:                  agent.ID,
			Name:                     "slow",
			Type:                     "http",
			Lifecycle:                "active",
			Health:                   "up",
			ReportingIntervalSeconds: 600,
		},
	}
	if err := database.Create(&monitors).Error; err != nil {
		t.Fatalf("create monitors: %v", err)
	}

	reportTime := time.Now().Add(-20 * time.Minute).UTC().Format(time.RFC3339)
	reports := []db.MonitorReport{
		{
			ID:          "report-fast",
			MonitorID:   "monitor-fast",
			Payload:     "{}",
			CollectedAt: reportTime,
			Health:      "up",
			CreatedAt:   time.Now().Add(-20 * time.Minute),
		},
		{
			ID:          "report-slow",
			MonitorID:   "monitor-slow",
			Payload:     "{}",
			CollectedAt: reportTime,
			Health:      "up",
			CreatedAt:   time.Now().Add(-20 * time.Minute),
		},
	}
	if err := database.Create(&reports).Error; err != nil {
		t.Fatalf("create reports: %v", err)
	}

	service := NewHealthService(database, logging.NewLogger())
	staleMonitors, err := service.DetectStaleMonitors(DefaultHealthConfig())
	if err != nil {
		t.Fatalf("DetectStaleMonitors() error = %v", err)
	}

	if len(staleMonitors) != 1 || staleMonitors[0].ID != "monitor-fast" {
		t.Fatalf("stale monitors = %+v, want only fast monitor stale", staleMonitors)
	}
}

func TestComputeMonitorHealthReturnsStaleForExpiredReport(t *testing.T) {
	database := setupHealthTestDB(t)
	agent := db.Agent{
		ID:        "agent-computed-stale",
		MachineId: "computed-stale-machine",
		Name:      "computed stale",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "computed-stale-token",
		LastSeen:  time.Now(),
	}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	monitor := db.Monitor{
		ID:                       "monitor-computed-stale",
		AgentID:                  agent.ID,
		Name:                     "computed stale",
		Type:                     "http",
		Lifecycle:                "active",
		Health:                   "up",
		ReportingIntervalSeconds: 60,
	}
	if err := database.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}

	reportTime := time.Now().Add(-20 * time.Minute).UTC()
	report := db.MonitorReport{
		ID:          "report-computed-stale",
		MonitorID:   monitor.ID,
		Payload:     "{}",
		CollectedAt: reportTime.Format(time.RFC3339),
		Health:      "up",
		CreatedAt:   reportTime,
	}
	if err := database.Create(&report).Error; err != nil {
		t.Fatalf("create report: %v", err)
	}

	service := NewHealthService(database, logging.NewLogger())
	health, err := service.ComputeMonitorHealth(monitor.ID, DefaultHealthConfig())
	if err != nil {
		t.Fatalf("ComputeMonitorHealth() error = %v", err)
	}
	if health != "stale" {
		t.Fatalf("ComputeMonitorHealth() = %q, want stale", health)
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
