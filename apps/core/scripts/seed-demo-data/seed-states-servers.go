package main

import (
	"fmt"
	"time"

	"orion/core/internal/db"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type showcaseServer struct {
	key, name, os, platform string
	labels                  []string
	notes                   string
	cpu, mem, disk          float64
	uptimeDays              int
	lastSeenAgo             time.Duration // zero means just now
	noReports               bool
	maintenanceHours        int // >0: in maintenance for this many more hours
	removed                 bool
	tokenState              string // "", rotated, grace, revoked
}

var showcaseServers = []showcaseServer{
	{key: "web-up", name: "web-prod-01", os: "linux", platform: "ubuntu", labels: []string{"env=prod", "role=web", "region=eu-west"}, notes: "Primary web node. Owner: platform team.", cpu: 23, mem: 41, disk: 58, uptimeDays: 41},
	{key: "api-degraded", name: "api-prod-02", os: "linux", platform: "ubuntu", labels: []string{"env=prod", "role=api"}, notes: "Elevated latency after the 14:10 deploy.", cpu: 91, mem: 78, disk: 64, uptimeDays: 9},
	{key: "db-down", name: "db-prod-01", os: "linux", platform: "debian", labels: []string{"env=prod", "role=database"}, cpu: 6, mem: 52, disk: 71, uptimeDays: 112},
	{key: "cache-maint", name: "cache-stage-01", os: "linux", platform: "ubuntu", labels: []string{"env=staging", "role=cache"}, notes: "Kernel patching window.", cpu: 12, mem: 33, disk: 40, uptimeDays: 3, maintenanceHours: 6},
	{key: "batch-stale", name: "batch-eu-03", os: "darwin", platform: "macOS", labels: []string{"env=prod", "role=batch"}, cpu: 18, mem: 44, disk: 52, uptimeDays: 20, lastSeenAgo: 40 * 24 * time.Hour},
	{key: "new-unknown", name: "edge-new-01", os: "linux", platform: "alpine", labels: []string{"env=prod", "role=edge"}, noReports: true},
	{key: "disk-full", name: "files-prod-01", os: "linux", platform: "ubuntu", labels: []string{"env=prod", "role=storage"}, notes: "Disk almost full. Cleanup ticket open.", cpu: 34, mem: 96, disk: 94, uptimeDays: 77},
	{key: "idle-low", name: "tools-dev-01", os: "darwin", platform: "macOS", labels: []string{"env=dev"}, cpu: 2, mem: 12, disk: 8, uptimeDays: 5},
	{key: "token-grace", name: "queue-prod-01", os: "linux", platform: "ubuntu", labels: []string{"env=prod", "role=queue"}, cpu: 31, mem: 47, disk: 45, uptimeDays: 14, tokenState: "grace"},
	{key: "token-rotated", name: "ci-runner-01", os: "linux", platform: "ubuntu", labels: []string{"env=ci"}, cpu: 45, mem: 60, disk: 61, uptimeDays: 30, tokenState: "rotated"},
	{key: "token-revoked", name: "legacy-app-01", os: "linux", platform: "centos", labels: []string{"env=prod"}, cpu: 9, mem: 30, disk: 44, uptimeDays: 200, tokenState: "revoked", lastSeenAgo: 9 * 24 * time.Hour},
	{key: "removed", name: "old-worker-07", os: "linux", platform: "ubuntu", labels: []string{"env=prod", "role=worker"}, cpu: 15, mem: 35, disk: 50, uptimeDays: 60, lastSeenAgo: 20 * 24 * time.Hour, removed: true},
}

func showcaseAgentID(key string) string { return "seed-state-agent-" + key }

func makeShowcaseAgent(i int, sv showcaseServer, now time.Time) db.Agent {
	seen := now.Add(-sv.lastSeenAgo - 2*time.Minute)
	if sv.lastSeenAgo == 0 && sv.noReports {
		seen = now.Add(-10 * time.Minute)
	}
	loc := db.GeoLocation{IP: fmt.Sprintf("203.0.113.%d", 20+i), Hostname: sv.name + ".example.com", City: "Dublin", Region: "Leinster", Country: "IE", Loc: "53.35,-6.26", Org: "Example Hosting", Timezone: "Europe/Dublin"}
	id := showcaseAgentID(sv.key)
	agent := db.Agent{
		ID: id, MachineId: "seed-state-machine-" + sv.key, Name: sv.name, OS: sv.os, Platform: sv.platform, KernelVersion: "6.8.0", Arch: "amd64",
		Token: "seed-state-token-" + sv.key, TokenHash: sha256Hex("seed-state-token-" + sv.key), TokenVersion: 1,
		Labels: mustJSON(sv.labels), Notes: sv.notes, ReportingIntervalSeconds: showcaseIntervalSeconds,
		CreatedAt: now.AddDate(0, 0, -120), LastSeen: seen, Location: datatypes.NewJSONType(loc), Meta: mustJSON(map[string]any{"seed": true, "showcase": sv.key}),
	}
	if sv.maintenanceHours > 0 {
		agent.MaintenanceMode, agent.MaintenanceUntil = true, ptrTime(now.Add(time.Duration(sv.maintenanceHours)*time.Hour))
	}
	switch sv.tokenState {
	case "grace":
		agent.TokenVersion, agent.TokenRotatedAt = 3, ptrTime(now.Add(-2*time.Minute))
		agent.PreviousTokenHash, agent.PreviousTokenExpiresAt = sha256Hex("seed-state-previous-token-"+sv.key), ptrTime(now.Add(8*time.Minute))
	case "rotated":
		agent.TokenVersion, agent.TokenRotatedAt = 2, ptrTime(now.AddDate(0, 0, -12))
	case "revoked":
		agent.TokenHash, agent.Token = "", "revoked:"+id
		agent.TokenVersion, agent.TokenRevokedAt, agent.TokenRevocationReason = 4, ptrTime(now.AddDate(0, 0, -2)), "Token leaked in a CI log"
	}
	if sv.removed {
		agent.Token, agent.TokenHash = "revoked:"+id, ""
		agent.TokenVersion, agent.TokenRevokedAt, agent.TokenRevocationReason = 2, ptrTime(now.AddDate(0, 0, -20)), "Server removed"
		agent.MachineId += "#removed-" + id
		agent.DeletedAt = now.AddDate(0, 0, -20)
	}
	return agent
}

// seedShowcaseServers creates the servers and their system reports; monitors come from seedShowcaseMonitors.
func seedShowcaseServers(database *gorm.DB, now time.Time) (map[string]db.Agent, error) {
	agents := map[string]db.Agent{}
	for i, sv := range showcaseServers {
		agent := makeShowcaseAgent(i, sv, now)
		if err := database.Create(&agent).Error; err != nil {
			return nil, err
		}
		agents[sv.key] = agent
		if sv.noReports {
			continue
		}
		reports := showcaseAgentReports(agent, sv.cpu, sv.mem, sv.disk, uint64(sv.uptimeDays)*86400, now, sv.lastSeenAgo+2*time.Minute)
		if err := database.CreateInBatches(reports, 100).Error; err != nil {
			return nil, err
		}
	}
	// A server that enrolled but never reported yet: a token that has never been used.
	return agents, seedEnrollmentSessions(database, now)
}

// seedEnrollmentSessions covers the Add server wizard states: waiting, claimed, expired, cancelled.
func seedEnrollmentSessions(database *gorm.DB, now time.Time) error {
	claimed := now.Add(-3 * time.Hour)
	cancelled := now.Add(-26 * time.Hour)
	sessions := []db.EnrollmentSession{
		{ID: "seed-enroll-waiting", TokenHash: sha256Hex("seed-enroll-waiting"), Platform: "linux", ExpiresAt: now.Add(8 * time.Minute), CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "seed-enroll-claimed", TokenHash: sha256Hex("seed-enroll-claimed"), Platform: "linux", MachineID: "seed-state-machine-web-up", AgentID: showcaseAgentID("web-up"), ClaimedAt: &claimed, ExpiresAt: now.Add(-170 * time.Minute), CreatedAt: now.Add(-190 * time.Minute)},
		{ID: "seed-enroll-expired", TokenHash: sha256Hex("seed-enroll-expired"), Platform: "darwin", ExpiresAt: now.Add(-25 * time.Hour), CreatedAt: now.Add(-25*time.Hour - 15*time.Minute)},
		{ID: "seed-enroll-cancelled", TokenHash: sha256Hex("seed-enroll-cancelled"), Platform: "linux", CancelledAt: &cancelled, ExpiresAt: now.Add(-25 * time.Hour), CreatedAt: now.Add(-26*time.Hour - 5*time.Minute)},
	}
	return database.Create(&sessions).Error
}
