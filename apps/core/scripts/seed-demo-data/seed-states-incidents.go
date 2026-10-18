package main

import (
	"fmt"
	"time"

	"orion/core/internal/db"

	"gorm.io/gorm"
)

type incidentEventSpec struct {
	typ, msg, actor, note string
	afterMin              int // minutes after the incident opened
}

type incidentSpec struct {
	key, title, status, severity string
	monitor                      string // monitor id; empty for declared incidents without one
	agent                        string
	source                       string
	openedAgoMin                 int
	resolvedAfterMin             int
	resolution                   string
	responder                    string
	summary                      string
	coveredBy                    string // incident key
	coverageNote                 string
	coveredForMin                int
	reopenCount                  int
	notification                 string
	events                       []incidentEventSpec
}

func agentMonitorID(server, key string) string { return "seed-state-monitor-" + server + "-" + key }
func coreMonitorID(key string) string          { return "seed-state-core-" + key }
func showcaseIncidentID(key string) string     { return "seed-state-incident-" + key }

var incidentSpecs = []incidentSpec{
	{key: "db-critical", title: "Postgres port is down", status: "open", severity: "critical", monitor: agentMonitorID("db-down", "pg"), agent: showcaseAgentID("db-down"), openedAgoMin: 190, responder: "Dana Whitfield", notification: "sent",
		summary: "The primary database refuses connections. Checkout and billing are failing.",
		events: []incidentEventSpec{{"incident_opened", "Incident opened", "core", "", 0}, {"monitor_failed", "Postgres port failed 3 checks in a row", "core", "", 2}, {"incident_responder_changed", "Responder set to Dana Whitfield", "admin", "", 6},
			{"incident_note", "Note added", "admin", "Disk on the primary filled up, postgres refuses to start. Freeing WAL space.", 14}, {"incident_note", "Note added", "admin", "Replica promotion is the fallback if cleanup does not work in 20 minutes.", 31}}},
	{key: "checkout-high", title: "Checkout API is down", status: "open", severity: "high", monitor: coreMonitorID("http-down"), agent: "seed-agent-core", openedAgoMin: 27, notification: "failed",
		events: []incidentEventSpec{{"incident_opened", "Incident opened", "core", "", 0}, {"monitor_failed", "Checkout API returned 503", "core", "", 1}}},
	{key: "memory-ack", title: "Memory is degraded", status: "acknowledged", severity: "medium", monitor: agentMonitorID("api-degraded", "mem"), agent: showcaseAgentID("api-degraded"), openedAgoMin: 310, responder: "Sam Okafor", notification: "sent",
		events: []incidentEventSpec{{"incident_opened", "Incident opened", "core", "", 0}, {"incident_acknowledged", "Incident manually acknowledged", "admin", "", 22}, {"incident_responder_changed", "Responder set to Sam Okafor", "admin", "", 24},
			{"incident_note", "Note added", "admin", "Leak suspected in the image resizer. Restarting workers every 2 hours until fixed.", 40}}},
	{key: "auth-reopened", title: "Auth service is down", status: "open", severity: "high", monitor: coreMonitorID("auth-down"), agent: "seed-agent-core", openedAgoMin: 900, reopenCount: 2, responder: "Priya Raman", notification: "sent",
		events: []incidentEventSpec{{"incident_opened", "Incident opened", "core", "", 0}, {"incident_resolved", "Monitor recovered", "core", "", 120}, {"incident_reopened", "Incident reopened", "core", "", 190}, {"incident_resolved", "Incident manually resolved", "admin", "Deploy rolled back.", 300},
			{"incident_reopened", "Incident reopened", "core", "", 640}, {"incident_note", "Note added", "admin", "Third failure this week. Opening a problem ticket for the token cache.", 660}}},
	{key: "billing-covered", title: "Billing API is down", status: "covered", severity: "high", monitor: coreMonitorID("maint-active"), agent: "seed-agent-core", openedAgoMin: 100, coveredBy: "db-critical", coverageNote: "Explained by the Postgres outage (INC above). Page again if still down after it recovers.", coveredForMin: 120, notification: "suppressed",
		events: []incidentEventSpec{{"incident_opened", "Incident opened", "core", "", 0}, {"incident_covered", "Incident marked covered", "admin", "Explained by the Postgres outage.", 12}}},
	{key: "hb-low", title: "Hourly invoice sync is late", status: "open", severity: "low", monitor: coreMonitorID("hb-late"), agent: "seed-agent-core", openedAgoMin: 130, notification: "sent",
		events: []incidentEventSpec{{"incident_opened", "Incident opened", "core", "", 0}, {"monitor_failed", "No heartbeat received for 3h", "core", "", 1}}},
	{key: "resolved-recovered", title: "web-prod-01 Website is degraded", status: "resolved", severity: "medium", monitor: agentMonitorID("web-up", "site"), agent: showcaseAgentID("web-up"), openedAgoMin: 2900, resolvedAfterMin: 45, resolution: "recovered", notification: "sent",
		events: []incidentEventSpec{{"incident_opened", "Incident opened", "core", "", 0}, {"incident_resolved", "Monitor recovered", "core", "", 45}}},
	{key: "resolved-manual", title: "Postgres primary is down", status: "resolved", severity: "high", monitor: coreMonitorID("tcp-up"), agent: "seed-agent-core", openedAgoMin: 5800, resolvedAfterMin: 95, resolution: "manual", responder: "Dana Whitfield", notification: "sent",
		summary: "Connection pool exhausted after a bad deploy.",
		events:  []incidentEventSpec{{"incident_opened", "Incident opened", "core", "", 0}, {"incident_note", "Note added", "admin", "Rolled back the deploy, pool is draining.", 40}, {"incident_resolved", "Incident manually resolved", "admin", "Confirmed healthy for 15 minutes.", 95}}},
	{key: "resolved-removed", title: "old-worker-07 Worker is down", status: "resolved", severity: "medium", monitor: "seed-state-monitor-removed-removed", agent: showcaseAgentID("removed"), openedAgoMin: 29000, resolvedAfterMin: 5, resolution: "monitor_removed", notification: "suppressed",
		events: []incidentEventSpec{{"incident_opened", "Incident opened", "core", "", 0}, {"incident_resolved", "Monitor removed; active incident resolved", "core", "", 5}}},
	{key: "resolved-decommissioned", title: "old-worker-07 is offline", status: "resolved", severity: "high", agent: showcaseAgentID("removed"), openedAgoMin: 30000, resolvedAfterMin: 10, resolution: "decommissioned", notification: "suppressed",
		events: []incidentEventSpec{{"incident_opened", "Incident opened", "core", "", 0}, {"incident_resolved", "Server removed", "admin", "", 10}}},
	{key: "declared-open", title: "Payments provider outage", status: "open", severity: "critical", source: "manual", openedAgoMin: 42, responder: "Priya Raman", notification: "sent",
		summary: "Our payments provider reports elevated errors. Card payments are failing for some customers.",
		events:  []incidentEventSpec{{"incident_declared", "Incident declared manually", "admin", "", 0}, {"incident_responder_changed", "Responder set to Priya Raman", "admin", "", 2}, {"incident_note", "Note added", "admin", "Provider status page confirms the issue. Switching new payments to the backup provider.", 18}, {"incident_public_update", "Public update posted: Investigating", "admin", "", 20}}},
	{key: "declared-resolved", title: "Slow checkout from Frankfurt", status: "resolved", severity: "medium", source: "manual", openedAgoMin: 4300, resolvedAfterMin: 210, resolution: "manual", responder: "Sam Okafor", notification: "sent",
		summary: "Support reported slow checkout since the CDN change.",
		events:  []incidentEventSpec{{"incident_declared", "Incident declared manually", "admin", "", 0}, {"incident_note", "Note added", "admin", "Rolled back the CDN rule for eu-west.", 60}, {"incident_resolved", "Incident manually resolved", "admin", "", 210}}},
	{key: "stale-runner", title: "Batch runner has stopped reporting", status: "open", severity: "high", monitor: agentMonitorID("batch-stale", "cron"), agent: showcaseAgentID("batch-stale"), openedAgoMin: 57600, notification: "sent",
		events: []incidentEventSpec{{"incident_opened", "Incident opened", "core", "", 0}, {"monitor_failed", "No report from batch-eu-03 for 40 days", "core", "", 1}}},
	{key: "stale-queue", title: "Batch queue port has stopped reporting", status: "open", severity: "high", monitor: agentMonitorID("batch-stale", "queue"), agent: showcaseAgentID("batch-stale"), openedAgoMin: 57600, notification: "sent",
		events: []incidentEventSpec{{"incident_opened", "Incident opened", "core", "", 0}}},
	{key: "ack-bulk-1", title: "Reports API latency is high", status: "acknowledged", severity: "medium", monitor: coreMonitorID("http-slow"), agent: "seed-agent-core", openedAgoMin: 500, responder: "Sam Okafor", notification: "sent",
		events: []incidentEventSpec{{"incident_opened", "Incident opened", "core", "", 0}, {"incident_acknowledged", "Incident manually acknowledged", "admin", "", 15}}},
}

func seedShowcaseIncidents(database *gorm.DB, now time.Time) error {
	var next int
	if err := database.Raw("SELECT COALESCE(MAX(number), 0) + 1 FROM incidents").Scan(&next).Error; err != nil {
		return err
	}
	numbers := map[string]int{}
	for i, spec := range incidentSpecs {
		numbers[spec.key] = next + i
	}
	for _, spec := range incidentSpecs {
		opened := now.Add(-time.Duration(spec.openedAgoMin) * time.Minute)
		inc := db.Incident{ID: showcaseIncidentID(spec.key), Number: numbers[spec.key], Status: spec.status, Severity: spec.severity, Title: spec.title, AgentID: spec.agent, MonitorID: spec.monitor,
			Source: choose(spec.source == "", "monitor", spec.source), Responder: spec.responder, Summary: spec.summary, OpenedAt: opened, ReopenCount: spec.reopenCount, NotificationStatus: spec.notification,
			ResolutionKind: spec.resolution, ImpactedComponents: "[]", CreatedAt: opened, UpdatedAt: now}
		if spec.responder != "" {
			inc.ResponderAt = ptrTime(opened.Add(6 * time.Minute))
		}
		last := opened
		for _, e := range spec.events {
			if at := opened.Add(time.Duration(e.afterMin) * time.Minute); at.After(last) {
				last, inc.LatestEvent = at, e.msg
			}
		}
		inc.LastEventAt = last
		if spec.resolvedAfterMin > 0 {
			inc.ResolvedAt = ptrTime(opened.Add(time.Duration(spec.resolvedAfterMin) * time.Minute))
		}
		if spec.reopenCount > 0 {
			inc.ReopenedAt = ptrTime(opened.Add(640 * time.Minute))
		}
		if spec.status == "covered" {
			inc.CoveredAt, inc.CoveredUntil, inc.CoverageNote = ptrTime(opened.Add(12*time.Minute)), ptrTime(now.Add(time.Duration(spec.coveredForMin)*time.Minute)), spec.coverageNote
			inc.CoveredByIncidentID = showcaseIncidentID(spec.coveredBy)
		}
		if err := database.Create(&inc).Error; err != nil {
			return err
		}
		events := make([]db.IncidentEvent, 0, len(spec.events))
		for i, e := range spec.events {
			events = append(events, db.IncidentEvent{ID: fmt.Sprintf("seed-state-incident-event-%s-%02d", spec.key, i+1), IncidentID: inc.ID, Type: e.typ, Message: e.msg,
				ActorType: choose(e.actor == "core", "system", "user"), ActorID: e.actor, Note: e.note, CreatedAt: opened.Add(time.Duration(e.afterMin) * time.Minute)})
		}
		if err := database.Create(&events).Error; err != nil {
			return err
		}
		if spec.monitor != "" && spec.status != "resolved" {
			if err := database.Model(&db.Monitor{}).Where("id = ?", spec.monitor).Update("active_incident_id", inc.ID).Error; err != nil {
				return err
			}
		}
	}
	// Candidates: failing monitors with no incident are open candidates (SMTP relay, Disk /data);
	// the Reports API one is dismissed with a reason.
	return database.Save(&db.IncidentCandidateDismissal{MonitorID: coreMonitorID("http-slow"), DismissedBy: "admin", Reason: "Known slow report exports on Mondays", DismissedAt: now.Add(-3 * time.Hour)}).Error
}
