package main

import (
	"fmt"
	"time"

	"orion/core/internal/db"

	"gorm.io/gorm"
)

type statusPageSeedStats struct {
	pages                int
	sections             int
	components           int
	mappings             int
	incidents            int
	updates              int
	subscribers          int
	subscriberComponents int
	deliveries           int
}

func seedStatusPages(database *gorm.DB, monitors []db.Monitor, now time.Time) (statusPageSeedStats, error) {
	stats := statusPageSeedStats{}
	monitorsByID := map[string]db.Monitor{}
	for _, monitor := range monitors {
		monitorsByID[monitor.ID] = monitor
	}

	publishedAt := now.Add(-7 * 24 * time.Hour)
	page := db.StatusPage{
		ID:                        "seed-status-page-main",
		Slug:                      "seed-orion-status",
		CustomDomain:              "status.seed-orion.local",
		Title:                     "Seed Orion Status",
		Description:               "Public-facing demo status page seeded from Orion monitor and incident data.",
		SEOTitle:                  "Seed Orion public service status",
		SEODescription:            "Demo operational status for seeded Orion services, components, incidents, and subscribers.",
		OpenGraphImageURL:         "https://status.example.test/og/seed-orion-status.png",
		CanonicalURL:              "https://status.example.test/status/seed-orion-status",
		Visibility:                "public",
		ThemeSettings:             mustJSON(map[string]any{"accent_color": "#2563eb", "mode": "system", "logo_url": "https://status.example.test/logo.svg"}),
		DefaultIncidentVisibility: "published",
		PublishedAt:               &publishedAt,
		CreatedAt:                 publishedAt,
		UpdatedAt:                 now,
	}
	if err := database.Create(&page).Error; err != nil {
		return stats, err
	}
	stats.pages++

	sections := []db.StatusPageSection{
		{ID: "seed-status-page-section-customer", StatusPageID: page.ID, Name: "Customer-facing systems", SortOrder: 1, CreatedAt: publishedAt, UpdatedAt: now},
		{ID: "seed-status-page-section-infra", StatusPageID: page.ID, Name: "Infrastructure", SortOrder: 2, CreatedAt: publishedAt, UpdatedAt: now},
		{ID: "seed-status-page-section-internal", StatusPageID: page.ID, Name: "Internal services", SortOrder: 3, CollapsedByDefault: true, CreatedAt: publishedAt, UpdatedAt: now},
	}
	if err := database.Create(&sections).Error; err != nil {
		return stats, err
	}
	stats.sections += len(sections)

	components := []db.StatusPageComponent{
		{
			ID:                "seed-status-page-component-public-api",
			StatusPageID:      page.ID,
			SectionID:         sections[0].ID,
			PublicName:        "Public API",
			PublicDescription: "Core API and console traffic served to customers.",
			DisplayMode:       "aggregate",
			SortOrder:         1,
			Visible:           true,
			CreatedAt:         publishedAt,
			UpdatedAt:         now,
		},
		{
			ID:                "seed-status-page-component-checkout",
			StatusPageID:      page.ID,
			SectionID:         sections[0].ID,
			PublicName:        "Checkout",
			PublicDescription: "Synthetic customer checkout flow with an active outage.",
			DisplayMode:       "single_resource",
			SortOrder:         2,
			Visible:           true,
			CreatedAt:         publishedAt,
			UpdatedAt:         now,
		},
		{
			ID:                "seed-status-page-component-worker",
			StatusPageID:      page.ID,
			SectionID:         sections[1].ID,
			PublicName:        "Worker Queue",
			PublicDescription: "Background job processing and resource pressure signals.",
			DisplayMode:       "aggregate",
			SortOrder:         1,
			Visible:           true,
			CreatedAt:         publishedAt,
			UpdatedAt:         now,
		},
		{
			ID:                 "seed-status-page-component-database",
			StatusPageID:       page.ID,
			SectionID:          sections[1].ID,
			PublicName:         "Database",
			PublicDescription:  "Manual component used to demonstrate non-monitor status overrides.",
			DisplayMode:        "manual",
			ManualStatus:       "operational",
			ManualStatusReason: "Seeded manual component is healthy.",
			SortOrder:          2,
			Visible:            true,
			CreatedAt:          publishedAt,
			UpdatedAt:          now,
		},
		{
			ID:                 "seed-status-page-component-maintenance",
			StatusPageID:       page.ID,
			SectionID:          sections[2].ID,
			PublicName:         "Planned Maintenance",
			PublicDescription:  "Public maintenance window used by the status page editor and public view.",
			DisplayMode:        "manual",
			ManualStatus:       "maintenance",
			ManualStatusReason: "Seeded maintenance window is in progress.",
			SortOrder:          1,
			Visible:            true,
			CreatedAt:          publishedAt,
			UpdatedAt:          now,
		},
		{
			ID:                "seed-status-page-component-private-admin",
			StatusPageID:      page.ID,
			SectionID:         sections[2].ID,
			PublicName:        "Private Admin API",
			PublicDescription: "Hidden component for testing private component and incident filtering.",
			DisplayMode:       "single_resource",
			SortOrder:         2,
			Visible:           false,
			CreatedAt:         publishedAt,
			UpdatedAt:         now,
		},
	}
	if err := database.Create(&components).Error; err != nil {
		return stats, err
	}
	if err := database.Model(&db.StatusPageComponent{}).
		Where("id = ?", "seed-status-page-component-private-admin").
		Update("visible", false).Error; err != nil {
		return stats, err
	}
	stats.components += len(components)

	mappings := statusPageSeedMappings(monitorsByID, now)
	if err := database.Create(&mappings).Error; err != nil {
		return stats, err
	}
	stats.mappings += len(mappings)

	incidents, updates := statusPageSeedIncidents(page.ID, now)
	if err := database.Create(&incidents).Error; err != nil {
		return stats, err
	}
	stats.incidents += len(incidents)
	if err := database.Create(&updates).Error; err != nil {
		return stats, err
	}
	stats.updates += len(updates)

	subscribers, subscriberComponents, deliveries := statusPageSeedSubscribers(page.ID, now)
	if err := database.Create(&subscribers).Error; err != nil {
		return stats, err
	}
	stats.subscribers += len(subscribers)
	if err := database.Create(&subscriberComponents).Error; err != nil {
		return stats, err
	}
	stats.subscriberComponents += len(subscriberComponents)
	if err := database.Create(&deliveries).Error; err != nil {
		return stats, err
	}
	stats.deliveries += len(deliveries)

	return stats, nil
}

func statusPageSeedMappings(monitorsByID map[string]db.Monitor, now time.Time) []db.StatusPageComponentMapping {
	mappingInputs := []struct {
		componentID string
		resourceID  string
		resourceTyp string
		health      string
		uptime      string
	}{
		{componentID: "seed-status-page-component-public-api", resourceID: "seed-monitor-core-public-api", resourceTyp: "monitor", health: "worst", uptime: "worst"},
		{componentID: "seed-status-page-component-public-api", resourceID: "seed-monitor-seed-agent-01-healthy-http", resourceTyp: "monitor", health: "worst", uptime: "average"},
		{componentID: "seed-status-page-component-checkout", resourceID: "seed-monitor-seed-agent-03-down-http", resourceTyp: "monitor", health: "worst", uptime: "worst"},
		{componentID: "seed-status-page-component-worker", resourceID: "seed-monitor-seed-agent-02-degraded-resource", resourceTyp: "monitor", health: "worst", uptime: "average"},
		{componentID: "seed-status-page-component-worker", resourceID: "seed-monitor-seed-agent-07-flapping-internal", resourceTyp: "monitor", health: "worst", uptime: "worst"},
		{componentID: "seed-status-page-component-maintenance", resourceID: "seed-monitor-seed-agent-04-maintenance-http", resourceTyp: "monitor", health: "manual", uptime: "manual"},
		{componentID: "seed-status-page-component-private-admin", resourceID: "seed-agent-10-alerts", resourceTyp: "agent", health: "worst", uptime: "worst"},
	}

	mappings := make([]db.StatusPageComponentMapping, 0, len(mappingInputs))
	for i, input := range mappingInputs {
		if input.resourceTyp == "monitor" {
			if _, ok := monitorsByID[input.resourceID]; !ok {
				continue
			}
		}
		mappings = append(mappings, db.StatusPageComponentMapping{
			ID:                   fmt.Sprintf("seed-status-page-mapping-%02d", i+1),
			ComponentID:          input.componentID,
			ResourceType:         input.resourceTyp,
			ResourceID:           input.resourceID,
			HealthRollupStrategy: input.health,
			UptimeRollupStrategy: input.uptime,
			CreatedAt:            now.Add(-7 * 24 * time.Hour),
			UpdatedAt:            now,
		})
	}
	return mappings
}

func statusPageSeedIncidents(pageID string, now time.Time) ([]db.StatusPageIncident, []db.StatusPageIncidentUpdate) {
	activePublishedAt := now.Add(-2 * time.Hour)
	resolvedPublishedAt := now.Add(-52 * time.Hour)
	resolvedAt := now.Add(-46 * time.Hour)
	scheduledStart := now.Add(6 * time.Hour)
	scheduledEnd := now.Add(8 * time.Hour)
	privatePublishedAt := now.Add(-30 * time.Minute)

	incidents := []db.StatusPageIncident{
		{
			ID:                   "seed-status-page-incident-checkout-outage",
			StatusPageID:         pageID,
			InternalIncidentID:   "seed-incident-seed-monitor-seed-agent-03-down-http",
			Title:                "Checkout API elevated errors",
			PublicStatus:         "identified",
			Severity:             "high",
			ImpactSummary:        "Checkout requests are failing for a subset of customers while the API monitor remains down.",
			Visibility:           "published",
			AffectedComponentIDs: mustJSON([]string{"seed-status-page-component-checkout"}),
			PublishedAt:          &activePublishedAt,
			CreatedAt:            activePublishedAt.Add(-10 * time.Minute),
			UpdatedAt:            now,
		},
		{
			ID:                   "seed-status-page-incident-worker-latency",
			StatusPageID:         pageID,
			InternalIncidentID:   "seed-incident-seed-monitor-seed-agent-02-degraded-resource",
			Title:                "Worker queue latency",
			PublicStatus:         "resolved",
			Severity:             "medium",
			ImpactSummary:        "Background jobs were delayed while worker hosts were under resource pressure.",
			Visibility:           "published",
			AffectedComponentIDs: mustJSON([]string{"seed-status-page-component-worker"}),
			PublishedAt:          &resolvedPublishedAt,
			ResolvedAt:           &resolvedAt,
			CreatedAt:            resolvedPublishedAt.Add(-20 * time.Minute),
			UpdatedAt:            resolvedAt,
		},
		{
			ID:                   "seed-status-page-incident-maintenance",
			StatusPageID:         pageID,
			Title:                "Scheduled database maintenance",
			PublicStatus:         "scheduled",
			Severity:             "low",
			ImpactSummary:        "A short maintenance window is scheduled for database patching.",
			Visibility:           "published",
			AffectedComponentIDs: mustJSON([]string{"seed-status-page-component-database", "seed-status-page-component-maintenance"}),
			PublishedAt:          ptrTime(now.Add(-24 * time.Hour)),
			ScheduledStartAt:     &scheduledStart,
			ScheduledEndAt:       &scheduledEnd,
			CreatedAt:            now.Add(-24 * time.Hour),
			UpdatedAt:            now,
		},
		{
			ID:                   "seed-status-page-incident-private-admin",
			StatusPageID:         pageID,
			InternalIncidentID:   "seed-incident-seed-monitor-seed-agent-10-alerts-http",
			Title:                "Private admin API investigation",
			PublicStatus:         "investigating",
			Severity:             "medium",
			ImpactSummary:        "Internal-only seeded incident used to verify private visibility boundaries.",
			Visibility:           "private",
			AffectedComponentIDs: mustJSON([]string{"seed-status-page-component-private-admin"}),
			PublishedAt:          &privatePublishedAt,
			CreatedAt:            privatePublishedAt,
			UpdatedAt:            now,
		},
		{
			ID:                   "seed-status-page-incident-draft",
			StatusPageID:         pageID,
			Title:                "Draft public update",
			PublicStatus:         "investigating",
			Severity:             "low",
			ImpactSummary:        "Draft seeded incident for the Console editor.",
			Visibility:           "draft",
			AffectedComponentIDs: mustJSON([]string{"seed-status-page-component-public-api"}),
			CreatedAt:            now.Add(-15 * time.Minute),
			UpdatedAt:            now.Add(-15 * time.Minute),
		},
	}

	updates := []db.StatusPageIncidentUpdate{
		{ID: "seed-status-page-update-checkout-1", IncidentID: "seed-status-page-incident-checkout-outage", Status: "investigating", Message: "We are investigating elevated checkout errors.", CreatedBy: "seed", PublishedAt: ptrTime(activePublishedAt), CreatedAt: activePublishedAt},
		{ID: "seed-status-page-update-checkout-2", IncidentID: "seed-status-page-incident-checkout-outage", Status: "identified", Message: "The failing dependency has been identified and traffic is being shifted.", CreatedBy: "seed", PublishedAt: ptrTime(now.Add(-70 * time.Minute)), CreatedAt: now.Add(-70 * time.Minute)},
		{ID: "seed-status-page-update-worker-1", IncidentID: "seed-status-page-incident-worker-latency", Status: "identified", Message: "Worker capacity was saturated during a batch import.", CreatedBy: "seed", PublishedAt: ptrTime(resolvedPublishedAt), CreatedAt: resolvedPublishedAt},
		{ID: "seed-status-page-update-worker-2", IncidentID: "seed-status-page-incident-worker-latency", Status: "resolved", Message: "Backlog cleared and worker latency returned to normal.", CreatedBy: "seed", PublishedAt: ptrTime(resolvedAt), CreatedAt: resolvedAt},
		{ID: "seed-status-page-update-maintenance-1", IncidentID: "seed-status-page-incident-maintenance", Status: "scheduled", Message: "Database maintenance is scheduled for tonight.", CreatedBy: "seed", PublishedAt: ptrTime(now.Add(-24 * time.Hour)), CreatedAt: now.Add(-24 * time.Hour)},
		{ID: "seed-status-page-update-private-1", IncidentID: "seed-status-page-incident-private-admin", Status: "investigating", Message: "Private admin-only update with no customer-facing details.", CreatedBy: "seed", PublishedAt: ptrTime(privatePublishedAt), CreatedAt: privatePublishedAt},
		{ID: "seed-status-page-update-draft-1", IncidentID: "seed-status-page-incident-draft", Status: "investigating", Message: "Draft update that should not appear on public endpoints.", CreatedBy: "seed", CreatedAt: now.Add(-15 * time.Minute)},
	}
	return incidents, updates
}

func statusPageSeedSubscribers(pageID string, now time.Time) ([]db.StatusPageSubscriber, []db.StatusPageSubscriberComponent, []db.StatusPageSubscriberDelivery) {
	confirmedAt := now.Add(-6 * 24 * time.Hour)
	lastDeliveryAt := now.Add(-70 * time.Minute)
	pendingExpiresAt := now.Add(24 * time.Hour)
	unsubscribedAt := now.Add(-18 * time.Hour)
	failedAt := now.Add(-68 * time.Minute)

	subscribers := []db.StatusPageSubscriber{
		{
			ID:                         "seed-status-page-subscriber-confirmed",
			StatusPageID:               pageID,
			DestinationType:            "email",
			DestinationHash:            "seed-destination-hash-confirmed",
			DestinationValueCiphertext: "seed-ciphertext-confirmed",
			MaskedDestination:          "al***@example.com",
			State:                      "confirmed",
			ConfirmationTokenHash:      "seed-confirmation-confirmed",
			ManageTokenHash:            "seed-manage-confirmed",
			ManageTokenVersion:         2,
			UnsubscribeTokenHash:       "seed-unsubscribe-confirmed",
			UnsubscribeTokenVersion:    2,
			LastDeliveryStatus:         "sent",
			LastDeliveryAt:             &lastDeliveryAt,
			Source:                     "public_page",
			ConfirmedAt:                &confirmedAt,
			CreatedAt:                  confirmedAt,
			UpdatedAt:                  now,
		},
		{
			ID:                         "seed-status-page-subscriber-scoped",
			StatusPageID:               pageID,
			DestinationType:            "email",
			DestinationHash:            "seed-destination-hash-scoped",
			DestinationValueCiphertext: "seed-ciphertext-scoped",
			MaskedDestination:          "ch***@example.com",
			State:                      "confirmed",
			ConfirmationTokenHash:      "seed-confirmation-scoped",
			ManageTokenHash:            "seed-manage-scoped",
			ManageTokenVersion:         2,
			UnsubscribeTokenHash:       "seed-unsubscribe-scoped",
			UnsubscribeTokenVersion:    2,
			LastDeliveryStatus:         "pending_sender_configuration",
			LastDeliveryAt:             &lastDeliveryAt,
			Source:                     "public_page",
			ConfirmedAt:                &confirmedAt,
			CreatedAt:                  confirmedAt,
			UpdatedAt:                  now,
		},
		{
			ID:                         "seed-status-page-subscriber-pending",
			StatusPageID:               pageID,
			DestinationType:            "email",
			DestinationHash:            "seed-destination-hash-pending",
			DestinationValueCiphertext: "seed-ciphertext-pending",
			MaskedDestination:          "pe***@example.com",
			State:                      "pending",
			ConfirmationTokenHash:      "seed-confirmation-pending",
			ConfirmationTokenExpiresAt: &pendingExpiresAt,
			ManageTokenHash:            "seed-manage-pending",
			UnsubscribeTokenHash:       "seed-unsubscribe-pending",
			Source:                     "public_page",
			CreatedAt:                  now.Add(-2 * time.Hour),
			UpdatedAt:                  now.Add(-2 * time.Hour),
		},
		{
			ID:                         "seed-status-page-subscriber-unsubscribed",
			StatusPageID:               pageID,
			DestinationType:            "email",
			DestinationHash:            "seed-destination-hash-unsubscribed",
			DestinationValueCiphertext: "seed-ciphertext-unsubscribed",
			MaskedDestination:          "un***@example.com",
			State:                      "unsubscribed",
			ConfirmationTokenHash:      "seed-confirmation-unsubscribed",
			ManageTokenHash:            "seed-manage-unsubscribed",
			UnsubscribeTokenHash:       "seed-unsubscribe-unsubscribed",
			Source:                     "public_page",
			ConfirmedAt:                ptrTime(now.Add(-10 * 24 * time.Hour)),
			UnsubscribedAt:             &unsubscribedAt,
			CreatedAt:                  now.Add(-10 * 24 * time.Hour),
			UpdatedAt:                  unsubscribedAt,
		},
	}

	subscriberComponents := []db.StatusPageSubscriberComponent{
		{ID: "seed-status-page-subscriber-component-confirmed-api", SubscriberID: "seed-status-page-subscriber-confirmed", ComponentID: "seed-status-page-component-public-api", EventScope: "all_updates", CreatedAt: confirmedAt, UpdatedAt: now},
		{ID: "seed-status-page-subscriber-component-confirmed-checkout", SubscriberID: "seed-status-page-subscriber-confirmed", ComponentID: "seed-status-page-component-checkout", EventScope: "all_updates", CreatedAt: confirmedAt, UpdatedAt: now},
		{ID: "seed-status-page-subscriber-component-scoped-checkout", SubscriberID: "seed-status-page-subscriber-scoped", ComponentID: "seed-status-page-component-checkout", EventScope: "all_updates", CreatedAt: confirmedAt, UpdatedAt: now},
	}

	deliveries := []db.StatusPageSubscriberDelivery{
		{
			ID:                     "seed-status-page-delivery-confirmed-checkout",
			SubscriberID:           "seed-status-page-subscriber-confirmed",
			StatusPageID:           pageID,
			PublicIncidentID:       "seed-status-page-incident-checkout-outage",
			PublicIncidentUpdateID: "seed-status-page-update-checkout-2",
			DeliveryType:           "email",
			DeliveryState:          "sent",
			ProviderMessageID:      "seed-provider-message-001",
			AttemptCount:           1,
			QueuedAt:               ptrTime(now.Add(-72 * time.Minute)),
			SentAt:                 &lastDeliveryAt,
			CreatedAt:              now.Add(-72 * time.Minute),
			UpdatedAt:              lastDeliveryAt,
		},
		{
			ID:                     "seed-status-page-delivery-scoped-checkout",
			SubscriberID:           "seed-status-page-subscriber-scoped",
			StatusPageID:           pageID,
			PublicIncidentID:       "seed-status-page-incident-checkout-outage",
			PublicIncidentUpdateID: "seed-status-page-update-checkout-2",
			DeliveryType:           "email",
			DeliveryState:          "pending_sender_configuration",
			ErrorCode:              "public_mail_sender_not_configured",
			SafeErrorSummary:       "Public status page mail sender is not configured.",
			AttemptCount:           1,
			QueuedAt:               ptrTime(now.Add(-72 * time.Minute)),
			FailedAt:               &failedAt,
			CreatedAt:              now.Add(-72 * time.Minute),
			UpdatedAt:              failedAt,
		},
	}
	return subscribers, subscriberComponents, deliveries
}
