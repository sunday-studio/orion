package soak

import (
	"context"
	"fmt"
	"orion/core/internal/api"
	"orion/core/internal/db"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
)

// waitForDrain waits until no alert delivery is still waiting to be sent or retried.
// With fastBackoff the retry delay (minutes in production) is skipped so a short run
// still exercises the retry path to completion.
func waitForDrain(database *gorm.DB, limit time.Duration, fastBackoff bool) {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if fastBackoff {
			database.Model(&db.AlertDelivery{}).Where("status = ?", "failed").Update("next_attempt_at", time.Now().UTC().Add(-time.Second))
		}
		var outstanding int64
		database.Model(&db.AlertDelivery{}).
			Where("status = ? OR (status = ? AND attempt_count < max_attempts)", "pending", "failed").
			Count(&outstanding)
		if outstanding == 0 {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func measureDatabase(ctx context.Context, database *gorm.DB, server *api.Server, dataDir string, result *Result) {
	if info, err := os.Stat(filepath.Join(dataDir, "orion.db")); err == nil {
		result.DatabaseBytes = info.Size()
	}
	if snapshot, err := server.RuntimeDiagnostics().Snapshot(ctx); err == nil {
		result.SQLiteBusy = snapshot.SQLite.BusyTotal
		result.ServerLatency = map[string]int64{}
		for route, p := range snapshot.RequestLatency {
			result.ServerLatency[route+" p50"] = p.P50
			result.ServerLatency[route+" p95"] = p.P95
		}
	}
}

// verify checks the invariants the product promises. Each problem is recorded as a
// violation string; an empty list means the run passed.
func verify(database *gorm.DB, hook *receiver, result *Result) {
	fail := func(format string, args ...any) {
		result.Violations = append(result.Violations, fmt.Sprintf(format, args...))
	}

	if result.ReportErrors > 0 {
		fail("%d of %d report requests failed (a locked database shows up here as HTTP 500)", result.ReportErrors, result.Reports)
	}
	if result.SQLiteBusy > 0 {
		fail("SQLITE_BUSY observed %d times", result.SQLiteBusy)
	}

	var totalIncidents int64
	database.Model(&db.Incident{}).Count(&totalIncidents)
	if int(totalIncidents) != len(result.Outages) {
		fail("incidents = %d, want exactly one per injected outage (%d): flapping or duplicates", totalIncidents, len(result.Outages))
	}

	hook.mu.Lock()
	defer hook.mu.Unlock()
	for _, o := range result.Outages {
		var incidents []db.Incident
		database.Where("monitor_id = ?", o.MonitorID).Find(&incidents)
		if len(incidents) != 1 {
			fail("monitor %s: %d incidents, want 1", o.MonitorID, len(incidents))
			continue
		}
		incident := incidents[0]
		if incident.Status != "resolved" {
			fail("monitor %s: incident %s is %q, want resolved after the outage ended", o.MonitorID, incident.ID, incident.Status)
		}
		for _, event := range []string{"incident_opened", "incident_resolved"} {
			var sent int64
			database.Model(&db.AlertDelivery{}).Where("incident_id = ? AND event_type = ? AND status = ?", incident.ID, event, "sent").Count(&sent)
			if sent != 1 {
				fail("incident %s: %d %s deliveries marked sent, want exactly 1", incident.ID, sent, event)
			}
			if got := hook.ok[event+"|"+incident.ID]; got != 1 {
				fail("incident %s: receiver accepted %s %d times, want exactly 1 (lost or duplicated alert)", incident.ID, event, got)
			}
		}
	}
}
