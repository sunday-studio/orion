package service

import (
	"fmt"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"testing"
	"time"
)

func BenchmarkStoreMonitorReport(b *testing.B) {
	database := testkit.NewDB(b)
	now := time.Now().UTC()
	node := db.Node{ID: "a", MachineId: "m", Name: "a", OS: "linux", Arch: "amd64", Token: "t", LastSeen: now}
	database.Create(&node)
	for i := 0; i < 1000; i++ {
		database.Create(&db.Monitor{ID: fmt.Sprintf("m%04d", i), NodeID: "a", Name: fmt.Sprintf("n%04d", i), Type: "http", Lifecycle: "active", Health: "up", ComputedHealth: "up", IncidentState: "up", ReportingIntervalSeconds: 30, CreatedAt: now})
	}
	svc := NewReportService(database, utils.NewLogger(), &config.Config{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("m%04d", i%1000)
		if _, err := svc.StoreMonitorReport(id, MonitorReportPayload{ReportID: fmt.Sprintf("r%d", i), Timestamp: now.Format(time.RFC3339), Health: "up", Metrics: map[string]any{}}); err != nil {
			b.Fatal(err)
		}
	}
}
