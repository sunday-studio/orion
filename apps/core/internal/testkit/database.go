// Package testkit provides shared fixtures for failure-mode tests: a real
// file-backed WAL SQLite, a controllable clock, a scriptable webhook receiver and
// a two-node fixture. It must not import service or api so any package can use it.
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

// NodeFixture is a node with one active monitor that last reported at ReportedAt.
type NodeFixture struct {
	Node       db.Node
	Monitor    db.Monitor
	ReportedAt time.Time
}

// TwoNodes creates two independent nodes, each owning one monitor with one
// historical report at `reportedAt`.
func TwoNodes(t testing.TB, database *gorm.DB, reportedAt time.Time) (NodeFixture, NodeFixture) {
	t.Helper()
	return makeNode(t, database, "a", reportedAt), makeNode(t, database, "b", reportedAt)
}

func makeNode(t testing.TB, database *gorm.DB, suffix string, reportedAt time.Time) NodeFixture {
	t.Helper()
	node := db.Node{ID: "node-" + suffix, MachineId: "machine-" + suffix, Name: "host-" + suffix, OS: "linux", Arch: "amd64", Token: "token-" + suffix, LastSeen: reportedAt}
	monitor := db.Monitor{ID: "monitor-" + suffix, NodeID: node.ID, Name: "site-" + suffix, Type: "http", Lifecycle: "active", Health: "up", ComputedHealth: "up", IncidentState: "up", ReportingIntervalSeconds: 60, CreatedAt: reportedAt}
	report := db.MonitorReport{ID: "report-" + suffix, MonitorID: monitor.ID, Payload: "{}", CollectedAt: reportedAt.Format(time.RFC3339), Health: "up", CreatedAt: reportedAt}
	for _, row := range []any{&node, &monitor, &report} {
		if err := database.Create(row).Error; err != nil {
			t.Fatalf("testkit: create fixture: %v", err)
		}
	}
	return NodeFixture{Node: node, Monitor: monitor, ReportedAt: reportedAt}
}
