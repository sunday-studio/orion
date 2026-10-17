// Package testkit provides shared fixtures for failure-mode tests: a real
// file-backed WAL SQLite, a controllable clock, a scriptable webhook receiver and
// a two-agent fixture. It must not import service or api so any package can use it.
package testkit

import (
	"orion/core/internal/db"
	"testing"
	"time"

	"gorm.io/gorm"
)

// NewDB opens a migrated, file-backed SQLite database using the same production
// profile as Core (WAL, busy timeout, pool), so concurrency tests behave like
// API plus Worker sharing one file.
func NewDB(t testing.TB) *gorm.DB {
	t.Helper()
	database, err := db.Initialize(t.TempDir(), "silent")
	if err != nil {
		t.Fatalf("testkit: open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("testkit: migrate database: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := database.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return database
}

// AgentFixture is an agent with one active monitor that last reported at ReportedAt.
type AgentFixture struct {
	Agent      db.Agent
	Monitor    db.Monitor
	ReportedAt time.Time
}

// TwoAgents creates two independent agents, each owning one monitor with one
// historical report at `reportedAt`.
func TwoAgents(t testing.TB, database *gorm.DB, reportedAt time.Time) (AgentFixture, AgentFixture) {
	t.Helper()
	return makeAgent(t, database, "a", reportedAt), makeAgent(t, database, "b", reportedAt)
}

func makeAgent(t testing.TB, database *gorm.DB, suffix string, reportedAt time.Time) AgentFixture {
	t.Helper()
	agent := db.Agent{ID: "agent-" + suffix, MachineId: "machine-" + suffix, Name: "host-" + suffix, OS: "linux", Arch: "amd64", Token: "token-" + suffix, LastSeen: reportedAt}
	monitor := db.Monitor{ID: "monitor-" + suffix, AgentID: agent.ID, Name: "site-" + suffix, Type: "http", Lifecycle: "active", Health: "up", ComputedHealth: "up", IncidentState: "up", ReportingIntervalSeconds: 60, CreatedAt: reportedAt}
	report := db.MonitorReport{ID: "report-" + suffix, MonitorID: monitor.ID, Payload: "{}", CollectedAt: reportedAt.Format(time.RFC3339), Health: "up", CreatedAt: reportedAt}
	for _, row := range []any{&agent, &monitor, &report} {
		if err := database.Create(row).Error; err != nil {
			t.Fatalf("testkit: create fixture: %v", err)
		}
	}
	return AgentFixture{Agent: agent, Monitor: monitor, ReportedAt: reportedAt}
}
