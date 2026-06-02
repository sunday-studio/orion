package main

import (
	"fmt"
	"strings"
	"time"

	"orion/core/internal/db"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func seedAlertChannels(database *gorm.DB, now time.Time) error {
	channels := []db.AlertChannel{
		{
			ID:               "seed-alert-channel-webhook-primary",
			Name:             "seed-webhook-primary",
			Type:             "webhook",
			Enabled:          true,
			WebhookURL:       "https://alerts.example.com/primary",
			SubscribedEvents: db.EncodeAlertEvents(db.DefaultAlertEvents()),
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "seed-alert-channel-webhook-secondary",
			Name:             "seed-webhook-secondary",
			Type:             "webhook",
			Enabled:          true,
			WebhookURL:       "https://alerts.example.com/secondary",
			SubscribedEvents: db.EncodeAlertEvents(db.DefaultAlertEvents()),
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}
	return database.Clauses(clause.OnConflict{DoNothing: true}).Create(&channels).Error
}

type incidentSeedStats struct {
	incidents       int
	incidentEvents  int
	alertDeliveries int
}

func seedIncidents(database *gorm.DB, monitors []db.Monitor, monitorToAgent map[string]db.Agent, monitorToScenario map[string]scenario, monitorToTemplate map[string]monitorTemplate, now time.Time) (incidentSeedStats, error) {
	stats := incidentSeedStats{}
	statuses := []string{"open", "acknowledged", "resolved"}
	alertStatuses := []string{"pending", "sent", "failed", "suppressed", "cooldown"}
	for _, monitor := range monitors {
		if monitor.Lifecycle != "active" || strings.Contains(monitor.Name, "Never Reported") {
			continue
		}
		sc := monitorToScenario[monitor.ID]
		tpl := monitorToTemplate[monitor.ID]
		if sc.status == "up" {
			continue
		}
		if sc.noReports {
			continue
		}
		agent := monitorToAgent[monitor.ID]
		status := statuses[(stats.incidents)%len(statuses)]
		if sc.status == "down" || sc.status == "alerts" {
			status = "open"
		}
		if sc.maintenance {
			status = "acknowledged"
		}
		openedAt := now.Add(-time.Duration(6+stats.incidents%72) * time.Hour)
		var resolvedAt *time.Time
		if status == "resolved" {
			resolvedAt = ptrTime(openedAt.Add(time.Duration(30+stats.incidents%240) * time.Minute))
		}
		health := currentHealth(sc, tpl)
		if health == "up" {
			health = "degraded"
		}
		incidentID := fmt.Sprintf("seed-incident-%s", monitor.ID)
		incident := db.Incident{
			ID:                 incidentID,
			Status:             status,
			Severity:           severityForHealth(health),
			Title:              fmt.Sprintf("%s is %s", monitor.Name, health),
			AgentID:            agent.ID,
			MonitorID:          monitor.ID,
			OpenedAt:           openedAt,
			ResolvedAt:         resolvedAt,
			LastEventAt:        incidentLastEventAt(resolvedAt, now.Add(-time.Duration(stats.incidents%90)*time.Minute)),
			LatestEvent:        fmt.Sprintf("Seeded %s incident for %s", health, monitor.Name),
			NotificationStatus: alertStatuses[stats.incidents%len(alertStatuses)],
			CreatedAt:          openedAt,
			UpdatedAt:          now,
		}
		if err := database.Create(&incident).Error; err != nil {
			return stats, err
		}
		stats.incidents++

		eventTypes := []string{"incident_opened", "monitor_failed"}
		if status == "resolved" {
			eventTypes = append(eventTypes, "incident_resolved")
		} else {
			eventTypes = append(eventTypes, "alert_rule_matched")
		}
		for i, eventType := range eventTypes {
			event := db.IncidentEvent{
				ID:              fmt.Sprintf("seed-incident-event-%s-%02d", incidentID, i+1),
				IncidentID:      incidentID,
				Type:            eventType,
				Message:         fmt.Sprintf("%s for %s", eventType, monitor.Name),
				MonitorReportID: "",
				CreatedAt:       openedAt.Add(time.Duration(i) * time.Minute),
			}
			if err := database.Create(&event).Error; err != nil {
				return stats, err
			}
			stats.incidentEvents++
		}
		for i, alertStatus := range alertStatuses {
			delivery := db.AlertDelivery{
				ID:         fmt.Sprintf("seed-alert-delivery-%s-%s", incidentID, alertStatus),
				IncidentID: incidentID,
				EventType:  choose(i%2 == 0, "incident_opened", "incident_resolved"),
				Channel:    choose(i%2 == 0, "seed-webhook-primary", "seed-webhook-secondary"),
				Type:       "webhook",
				Status:     alertStatus,
				Error:      choose(alertStatus == "failed", "seeded webhook delivery failure: connection refused", ""),
				CreatedAt:  openedAt.Add(time.Duration(i) * time.Minute),
				UpdatedAt:  openedAt.Add(time.Duration(i+1) * time.Minute),
			}
			if err := database.Create(&delivery).Error; err != nil {
				return stats, err
			}
			stats.alertDeliveries++
		}
		if incident.Status != "resolved" {
			if err := database.Model(&db.Monitor{}).Where("id = ?", monitor.ID).Updates(map[string]any{
				"active_incident_id": incident.ID,
				"incident_state":     incidentState(health),
			}).Error; err != nil {
				return stats, err
			}
		}
	}
	return stats, nil
}

func incidentState(health string) string {
	switch health {
	case "up", "down", "degraded", "stale":
		return health
	default:
		return "unknown"
	}
}

func severityForHealth(health string) string {
	switch health {
	case "down", "stale":
		return "high"
	case "degraded", "unknown":
		return "medium"
	default:
		return "low"
	}
}

func incidentLastEventAt(resolvedAt *time.Time, fallback time.Time) time.Time {
	if resolvedAt != nil {
		return *resolvedAt
	}
	return fallback
}
