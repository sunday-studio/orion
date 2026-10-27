package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func benchEnvInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}

// seedUptimeBench fills a file-backed database with monitors and days of raw reports, then rolls up
// every completed day the way the nightly job does, so list and overview run against production-shaped data.
func seedUptimeBench(b *testing.B, monitors, days, intervalSeconds int) *Server {
	b.Helper()
	gin.SetMode(gin.TestMode)
	database, err := db.Initialize(b.TempDir(), "silent")
	if err != nil {
		b.Fatal(err)
	}
	if err := db.Migrate(database); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if sqlDB, err := database.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now().UTC()
	node := db.Node{ID: "n1", MachineId: "m", Name: "n1", OS: "linux", Arch: "amd64", Token: "t", LastSeen: now}
	if err := database.Create(&node).Error; err != nil {
		b.Fatal(err)
	}
	for i := 0; i < monitors; i++ {
		m := db.Monitor{ID: fmt.Sprintf("m%04d", i), NodeID: "n1", Name: fmt.Sprintf("monitor-%04d", i), Type: "http", Lifecycle: "active",
			Health: "up", ComputedHealth: "up", IncidentState: "up", ReportingIntervalSeconds: intervalSeconds, CreatedAt: now.AddDate(0, 0, -days-1)}
		if err := database.Create(&m).Error; err != nil {
			b.Fatal(err)
		}
	}

	perMonitor := days * 86400 / intervalSeconds
	payload := `{"runner":"node","type":"http","status_code":200,"duration_ms":201}`
	start := now.Add(-time.Duration(days) * 24 * time.Hour).Unix()
	seed := fmt.Sprintf(`WITH RECURSIVE seq(i) AS (SELECT 0 UNION ALL SELECT i+1 FROM seq WHERE i < %d)
		INSERT INTO monitor_reports (id, monitor_id, payload, collected_at, health, client_report_id, created_at)
		SELECT m.id || '-' || seq.i, m.id, ?, strftime('%%Y-%%m-%%dT%%H:%%M:%%SZ', %d + seq.i*%d, 'unixepoch'),
		  CASE WHEN (seq.i %% 97) = 0 THEN 'down' ELSE 'up' END, '',
		  strftime('%%Y-%%m-%%d %%H:%%M:%%S', %d + seq.i*%d, 'unixepoch') || '+00:00'
		FROM monitors m, seq`, perMonitor-1, start, intervalSeconds, start, intervalSeconds)
	if err := database.Exec(seed, payload).Error; err != nil {
		b.Fatal(err)
	}

	rollups := service.NewRollupService(database, utils.NewLogger())
	for d := days; d >= 1; d-- {
		if _, err := rollups.RollupMonitorUptimeDay(now.AddDate(0, 0, -d)); err != nil {
			b.Fatal(err)
		}
	}
	var rows int64
	database.Model(&db.MonitorReport{}).Count(&rows)
	b.Logf("seeded %d monitors, %d days, %ds interval: %d raw reports", monitors, days, intervalSeconds, rows)
	return NewServer(database, utils.NewLogger(), &config.Config{})
}

// BenchmarkUptimeEndpoints measures the Console pages the uptime rollups were added for, the detail
// uptime endpoints (a status page component calls the same service methods per mapped monitor or node),
// and the same endpoints after every rollup is deleted, which forces the raw-report fallback.
// Scale it with ORION_BENCH_MONITORS, ORION_BENCH_DAYS and ORION_BENCH_INTERVAL (seconds).
func BenchmarkUptimeEndpoints(b *testing.B) {
	server := seedUptimeBench(b, benchEnvInt("ORION_BENCH_MONITORS", 100), benchEnvInt("ORION_BENCH_DAYS", 90), benchEnvInt("ORION_BENCH_INTERVAL", 900))
	paths := []string{"/v1/monitors", "/v1/overview", "/v1/monitors/m0000/uptime?period=90d", "/v1/nodes/n1/uptime?period=90d"}
	runAll := func(prefix string) {
		for _, path := range paths {
			b.Run(prefix+path, func(b *testing.B) {
				w := httptest.NewRecorder()
				server.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
				if w.Code != http.StatusOK {
					b.Fatalf("%s = %d %s", path, w.Code, w.Body.String())
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					server.router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
				}
			})
		}
	}
	runAll("rollups")
	if err := server.db.Exec("DELETE FROM monitor_uptime_rollups").Error; err != nil {
		b.Fatal(err)
	}
	runAll("no-rollups")
}
