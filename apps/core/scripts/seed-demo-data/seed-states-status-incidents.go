package main

import (
	"time"

	"orion/core/internal/db"

	"gorm.io/gorm"
)

type publicUpdateSpec struct {
	status, message string
	agoMin          int
}

type publicIncidentSpec struct {
	key, title, status, severity, visibility, summary string
	affected                                          []string
	internal                                          string // showcase incident key
	scheduledStartIn, scheduledEndIn                  int    // minutes from now, for scheduled maintenance
	resolvedAgoMin                                    int
	updates                                           []publicUpdateSpec
}

func seedShowcasePublicIncidents(database *gorm.DB, now time.Time) error {
	specs := []publicIncidentSpec{
		{key: "payments", title: "Payments provider outage", status: "investigating", severity: "critical", visibility: "published", summary: "Card payments fail for some customers.", affected: []string{"payments"}, internal: "declared-open",
			updates: []publicUpdateSpec{{"investigating", "We are investigating failures reported by our payments provider.", 38}, {"investigating", "The provider confirmed an incident on their side. New payments are being routed to a backup provider.", 20}}},
		{key: "storage", title: "File uploads failing in EU", status: "identified", severity: "high", visibility: "published", summary: "Uploads fail for some customers in Europe.", affected: []string{"storage"},
			updates: []publicUpdateSpec{{"investigating", "Reports of failed uploads in EU regions.", 150}, {"identified", "A faulty storage node was identified and is being drained.", 95}}},
		{key: "webhooks", title: "Delayed webhook deliveries", status: "monitoring", severity: "medium", visibility: "published", summary: "Webhooks arrive up to 5 minutes late.", affected: []string{"webhooks"},
			updates: []publicUpdateSpec{{"investigating", "Webhook deliveries are delayed.", 300}, {"identified", "Queue backlog caused by a spike in events.", 260}, {"monitoring", "Capacity added; the backlog is draining.", 40}}},
		{key: "api-resolved", title: "Elevated API errors", status: "resolved", severity: "high", visibility: "published", summary: "Resolved after a rollback.", affected: []string{"api", "dashboard"}, internal: "resolved-manual", resolvedAgoMin: 5700,
			updates: []publicUpdateSpec{{"investigating", "We see elevated 5xx errors on the API.", 5800}, {"identified", "A bad deploy is the cause; rolling back.", 5760}, {"monitoring", "Rollback complete, error rate back to normal.", 5720}, {"resolved", "This incident has been resolved.", 5700}}},
		{key: "maintenance", title: "Database minor version upgrade", status: "scheduled", severity: "low", visibility: "published", summary: "A short read-only period is expected.", affected: []string{"database", "api"}, scheduledStartIn: 1800, scheduledEndIn: 1920,
			updates: []publicUpdateSpec{{"scheduled", "We will upgrade the database. Expect read-only mode for about 10 minutes.", 1500}}},
		{key: "draft", title: "Investigating login slowness", status: "investigating", severity: "low", visibility: "draft", summary: "Draft, not public yet.", affected: []string{"dashboard"},
			updates: []publicUpdateSpec{{"investigating", "Draft update waiting to be published.", 12}}},
		{key: "private", title: "Internal credential rotation", status: "investigating", severity: "medium", visibility: "private", summary: "Internal only.", affected: []string{"internal"},
			updates: []publicUpdateSpec{{"investigating", "Internal note, never shown publicly.", 600}}},
	}
	for _, sp := range specs {
		id := "seed-state-pubinc-" + sp.key
		first := sp.updates[0].agoMin
		var affected []string
		for _, a := range sp.affected {
			affected = append(affected, "seed-state-component-"+a)
		}
		inc := db.StatusPageIncident{ID: id, StatusPageID: acmePageID, Title: sp.title, PublicStatus: sp.status, Severity: sp.severity, ImpactSummary: sp.summary, Visibility: sp.visibility,
			AffectedComponentIDs: mustJSON(affected), CreatedAt: now.Add(-time.Duration(first) * time.Minute), UpdatedAt: now.Add(-time.Duration(sp.updates[len(sp.updates)-1].agoMin) * time.Minute)}
		if sp.internal != "" {
			inc.InternalIncidentID = showcaseIncidentID(sp.internal)
		}
		if sp.visibility == "published" {
			inc.PublishedAt = ptrTime(now.Add(-time.Duration(first) * time.Minute))
		}
		if sp.resolvedAgoMin > 0 {
			inc.ResolvedAt = ptrTime(now.Add(-time.Duration(sp.resolvedAgoMin) * time.Minute))
		}
		if sp.scheduledStartIn > 0 {
			inc.ScheduledStartAt, inc.ScheduledEndAt = ptrTime(now.Add(time.Duration(sp.scheduledStartIn)*time.Minute)), ptrTime(now.Add(time.Duration(sp.scheduledEndIn)*time.Minute))
		}
		if err := database.Create(&inc).Error; err != nil {
			return err
		}
		for i, u := range sp.updates {
			at := now.Add(-time.Duration(u.agoMin) * time.Minute)
			upd := db.StatusPageIncidentUpdate{ID: id[:len("seed-state-pub")] + "upd-" + sp.key + "-" + string(rune('1'+i)), IncidentID: id, Status: u.status, Message: u.message, CreatedBy: "admin", CreatedAt: at}
			if sp.visibility != "draft" {
				upd.PublishedAt = &at
			}
			if err := database.Create(&upd).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
