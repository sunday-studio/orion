package main

import (
	"fmt"
	"time"

	"orion/core/internal/db"

	"gorm.io/gorm"
)

type auditSpec struct {
	agoMin                              int
	actor, actorType                    string
	action, objectType, objectID, label string
}

func seedShowcaseSettings(database *gorm.DB, now time.Time) error {
	// Instance settings: only fill an empty row, never overwrite what an operator saved.
	var existing int64
	database.Model(&db.InstanceSettings{}).Where("id = 1").Count(&existing)
	if existing == 0 {
		if err := database.Create(&db.InstanceSettings{ID: 1, InstanceName: "Acme Orion", Timezone: "UTC", TimeFormat: "24", UpdatedBy: "admin", UpdatedAt: now.AddDate(0, 0, -20)}).Error; err != nil {
			return err
		}
	}
	tokens := []db.APIToken{
		{ID: "seed-api-token-ci", Name: "ci-deploys", Prefix: "orn_api_3Fk9", TokenHash: sha256Hex("seed-api-ci"), Scope: "write", CreatedBy: "admin", CreatedAt: now.AddDate(0, 0, -16), LastUsedAt: ptrTime(now.Add(-40 * time.Minute))},
		{ID: "seed-api-token-grafana", Name: "grafana", Prefix: "orn_api_9aB2", TokenHash: sha256Hex("seed-api-grafana"), Scope: "read", CreatedBy: "admin", CreatedAt: now.AddDate(0, 0, -37), LastUsedAt: ptrTime(now.Add(-3 * 24 * time.Hour))},
		{ID: "seed-api-token-scripts", Name: "backup-script", Prefix: "orn_api_77cD", TokenHash: sha256Hex("seed-api-scripts"), Scope: "read", CreatedBy: "admin", CreatedAt: now.AddDate(0, 0, -2)},
		{ID: "seed-api-token-old", Name: "old-integration", Prefix: "orn_api_00ee", TokenHash: sha256Hex("seed-api-old"), Scope: "write", CreatedBy: "admin", CreatedAt: now.AddDate(0, 0, -120), LastUsedAt: ptrTime(now.AddDate(0, 0, -100)), RevokedAt: ptrTime(now.AddDate(0, 0, -90))},
	}
	if err := database.Create(&tokens).Error; err != nil {
		return err
	}
	specs := []auditSpec{
		{20, "admin", "user", "incident_resolved", "incident", showcaseIncidentID("resolved-manual"), "INC-0231"},
		{54, "admin", "user", "incident_acknowledged", "incident", showcaseIncidentID("memory-ack"), "INC-0231"},
		{130, "admin", "user", "incident_note_added", "incident", showcaseIncidentID("db-critical"), "INC-0230"},
		{210, "admin", "user", "monitor_paused", "monitor", coreMonitorID("paused"), "Staging API (paused)"},
		{390, "ci-deploys", "api_token", "monitor_created", "monitor", coreMonitorID("http-up"), "Marketing site"},
		{600, "ci-deploys", "api_token", "monitor_updated", "monitor", coreMonitorID("http-up"), "Marketing site"},
		{1500, "admin", "user", "node_token_rotated", "node", showcaseNodeID("token-grace"), "queue-prod-01"},
		{1620, "admin", "user", "alert_channel_created", "alert_channel", "seed-state-ch-pager", "pager-prod webhook"},
		{1700, "admin", "user", "alert_rule_updated", "alert_rule", "seed-state-rule-pager", "Critical to pager"},
		{2900, "admin", "user", "api_token_created", "api_token", "seed-api-token-ci", "ci-deploys"},
		{3100, "admin", "user", "settings_general_updated", "setting", "", ""},
		{4300, "admin", "user", "app_created", "app", appIDFor("billing-api"), "billing-api"},
		{4310, "admin", "user", "app_key_created", "app", appIDFor("billing-api"), "billing-api"},
		{5600, "admin", "user", "status_page_created", "status_page", acmePageID, "Acme Cloud Status"},
		{5900, "admin", "user", "status_page_maintenance_scheduled", "status_page", acmePageID, "Acme Cloud Status"},
		{7200, "ci-deploys", "api_token", "incident_declared", "incident", showcaseIncidentID("declared-open"), "Payments provider outage"},
		{8600, "admin", "user", "monitor_deleted", "monitor", "seed-state-core-gone", "Old health check"},
		{10000, "admin", "user", "data_lifecycle_rollup_run", "data_lifecycle", "settings", ""},
		{12000, "admin", "user", "node_decommissioned", "node", showcaseNodeID("removed"), "old-worker-07"},
		{14000, "admin", "user", "app_key_rotated", "app", appIDFor("billing-api"), "billing-api"},
		{16500, "grafana", "api_token", "monitor_updated", "monitor", coreMonitorID("http-slow"), "Reports API"},
		{19000, "admin", "user", "registration_token_rotated", "setting", "", ""},
		{21000, "admin", "user", "alert_route_created", "alert_route", "seed-label-route-1", "Route 1"},
		{240, "admin", "user", "incident_covered", "incident", showcaseIncidentID("billing-covered"), "Billing API"},
	}
	for i, a := range specs {
		meta := fmt.Sprintf(`{"target_label":%q,"method":"POST","route":"/v1/..."}`, a.label)
		pageID := ""
		if a.objectType == "status_page" {
			pageID = a.objectID
		}
		event := db.AuditEvent{ID: fmt.Sprintf("seed-audit-%02d", i+1), Action: a.action, StatusPageID: pageID, AffectedObjectType: a.objectType, AffectedObjectID: a.objectID, ActorType: a.actorType, ActorID: a.actor, MetadataJSON: meta, CreatedAt: now.Add(-time.Duration(a.agoMin) * time.Minute)}
		if err := database.Create(&event).Error; err != nil {
			return err
		}
	}
	return nil
}
