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
