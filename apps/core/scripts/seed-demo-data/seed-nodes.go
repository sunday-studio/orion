package main

import (
	"fmt"
	"time"

	"orion/core/internal/db"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func scenarioForIndex(i int) scenario {
	if i < len(scenarios) {
		return scenarios[i]
	}
	base := scenarios[i%len(scenarios)]
	base.key = fmt.Sprintf("%s-%02d", base.key, i+1)
	base.name = fmt.Sprintf("%s %02d", base.name, i+1)
	if i%11 == 0 {
		base.maintenance = true
		base.status = "maintenance"
	}
	if i%13 == 0 {
		base.stale = true
		base.status = "stale"
		base.noReports = false
	}
	return base
}

func makeNode(index int, sc scenario, now time.Time) db.Node {
	lastSeen := now.Add(-time.Duration(index%8) * time.Minute)
	if sc.stale {
		lastSeen = staleSince(now, index)
	}
	if sc.noReports {
		lastSeen = now.Add(-time.Duration(index%6) * time.Minute)
	}
	if scenarioBaseKey(sc) == "down" || scenarioBaseKey(sc) == "alerts" {
		lastSeen = now.Add(-time.Duration(index%3) * time.Minute)
	}
	location := db.GeoLocation{
		IP:       fmt.Sprintf("100.64.%d.%d", index/200, 10+index),
		Hostname: fmt.Sprintf("seed-%s.local", sc.key),
		City:     "Home Lab",
		Region:   "Local",
		Country:  "US",
		Loc:      "0,0",
		Org:      "Orion Seed",
		Timezone: "UTC",
	}
	meta := mustJSON(map[string]any{
		"seed":     true,
		"scenario": sc.key,
		"status":   sc.status,
	})
	return db.Node{
		ID:                       fmt.Sprintf("seed-node-%02d-%s", index+1, sc.key),
		MachineId:                fmt.Sprintf("seed-machine-%02d-%s", index+1, sc.key),
		Name:                     sc.name,
		OS:                       choose(index%3 == 0, "darwin", "linux"),
		Platform:                 choose(index%3 == 0, "macOS", "ubuntu"),
		KernelVersion:            choose(index%3 == 0, "23.6.0", "6.8.0"),
		Arch:                     choose(index%2 == 0, "arm64", "amd64"),
		Token:                    fmt.Sprintf("seed-token-%02d-%s", index+1, sc.key),
		MaintenanceMode:          sc.maintenance,
		ReportingIntervalSeconds: showcaseIntervalSeconds,
		CreatedAt:                now.AddDate(0, 0, -120),
		LastSeen:                 lastSeen,
		Location:                 datatypes.NewJSONType(location),
		Meta:                     meta,
	}
}

func makeMonitors(node db.Node, sc scenario, now time.Time) []db.Monitor {
	monitors := make([]db.Monitor, 0, len(monitorTemplates)+3)
	for i, tpl := range monitorTemplates {
		health := currentHealth(sc, tpl)
		var lastSuccess *time.Time
		if health == "up" || health == "degraded" {
			t := now.Add(-time.Duration(i+1) * time.Minute)
			lastSuccess = &t
		}
		description := tpl.description
		monitors = append(monitors, db.Monitor{
			ID:                       fmt.Sprintf("seed-monitor-%s-%s", node.ID, tpl.key),
			Description:              &description,
			Type:                     tpl.kind,
			Name:                     fmt.Sprintf("%s %s", sc.name, tpl.name),
			NodeID:                   node.ID,
			LastSuccessfulReportAt:   lastSuccess,
			ReportingIntervalSeconds: showcaseIntervalSeconds,
			ComputedHealth:           health,
			LastHealthComputation:    ptrTime(now.Add(-time.Duration(i) * time.Minute)),
			Lifecycle:                "active",
			Health:                   health,
			IncidentState:            incidentState(health),
			Meta: mustJSON(map[string]any{
				"seed":      true,
				"scenario":  sc.key,
				"monitor":   tpl.key,
				"edge_case": edgeCase(sc, tpl),
			}),
			CreatedAt: choose(sc.noReports, now.Add(-2*time.Hour), now.AddDate(0, 0, -120)),
			UpdatedAt: now.Add(-time.Duration(i) * time.Minute),
		})
	}

	disabledDescription := "Disabled monitor for lifecycle filtering"
	deletedDescription := "Deleted monitor for lifecycle filtering"
	monitors = append(monitors,
		db.Monitor{
			ID:          fmt.Sprintf("seed-monitor-%s-disabled", node.ID),
			Description: &disabledDescription,
			Type:        "http-healthcheck",
			Name:        fmt.Sprintf("%s Disabled Monitor", sc.name),
			NodeID:      node.ID,
			Lifecycle:   "disabled",
			Health:      "unknown",
			Meta:        mustJSON(map[string]any{"seed": true, "scenario": sc.key, "lifecycle": "disabled"}),
			CreatedAt:   now.AddDate(0, 0, -80),
			UpdatedAt:   now.AddDate(0, 0, -10),
		},
		db.Monitor{
			ID:          fmt.Sprintf("seed-monitor-%s-deleted", node.ID),
			Description: &deletedDescription,
			Type:        "command",
			Name:        fmt.Sprintf("%s Deleted Monitor", sc.name),
			NodeID:      node.ID,
			Lifecycle:   "deleted",
			Health:      "unknown",
			Meta:        mustJSON(map[string]any{"seed": true, "scenario": sc.key, "lifecycle": "deleted"}),
			CreatedAt:   now.AddDate(0, 0, -70),
			UpdatedAt:   now.AddDate(0, 0, -20),
			DeletedAt:   now.AddDate(0, 0, -20),
		},
	)

	if sc.noReports {
		neverDescription := "Active monitor with no reports for unknown state"
		monitors = append(monitors, db.Monitor{
			ID:                       fmt.Sprintf("seed-monitor-%s-never-reported", node.ID),
			Description:              &neverDescription,
			Type:                     "tcp",
			Name:                     fmt.Sprintf("%s Never Reported", sc.name),
			NodeID:                   node.ID,
			ReportingIntervalSeconds: showcaseIntervalSeconds,
			Lifecycle:                "active",
			Health:                   "unknown",
			Meta:                     mustJSON(map[string]any{"seed": true, "scenario": sc.key, "edge_case": "never_reported"}),
			CreatedAt:                now.Add(-2 * time.Hour),
			UpdatedAt:                now.Add(-2 * time.Hour),
		})
	}
	return monitors
}

func makeCoreOwner(now time.Time) db.Node {
	return db.Node{
		ID:                       "seed-node-core",
		System:                   true,
		MachineId:                "seed-machine-core",
		Name:                     "Orion Core",
		OS:                       "linux",
		Platform:                 "orion",
		KernelVersion:            "core",
		Arch:                     "amd64",
		Token:                    "seed-token-core",
		ReportingIntervalSeconds: 60,
		CreatedAt:                now.AddDate(0, 0, -120),
		LastSeen:                 now,
		Meta: mustJSON(map[string]any{
			"seed":  true,
			"owner": "core",
		}),
	}
}

func makeCoreMonitor(node db.Node, now time.Time) db.Monitor {
	description := "Core-managed HTTP check seeded for Console owner and source filters"
	lastSuccess := now.Add(-time.Minute)
	return db.Monitor{
		ID:                       "seed-monitor-core-public-api",
		Description:              &description,
		Type:                     "http",
		Name:                     "Core Public API",
		NodeID:                   node.ID,
		LastSuccessfulReportAt:   &lastSuccess,
		ReportingIntervalSeconds: 60,
		ComputedHealth:           "up",
		LastHealthComputation:    ptrTime(now),
		Lifecycle:                "active",
		Health:                   "up",
		IncidentState:            "unknown",
		Meta: mustJSON(map[string]any{
			"seed":    true,
			"owner":   "core",
			"monitor": "core-http",
		}),
		CreatedAt: now.AddDate(0, 0, -120),
		UpdatedAt: now,
	}
}

func seedCoreMonitorConfig(database *gorm.DB, monitorID string, now time.Time) error {
	return database.Create(&db.CoreMonitorConfig{
		MonitorID:       monitorID,
		Kind:            "http",
		ConfigJSON:      mustJSON(map[string]any{"url": "https://status.example.test/health", "expected_status": 200}),
		SecretRefJSON:   "{}",
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		NextRunAt:       farFuture(now),
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error
}

// staleSince is when a "stale" scenario last reported: older than five days, the staleness window of
// the one day reporting interval, so it stays stale for days instead of only for the first minutes.
func staleSince(now time.Time, jitter int) time.Time {
	return now.Add(-6*24*time.Hour - time.Duration(jitter%72)*time.Minute)
}
