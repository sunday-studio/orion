package service

import (
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"testing"
	"time"

	"gorm.io/gorm"
)

// groupingFixture has a webhook channel and an alert service whose webhook always succeeds.
func groupingFixture(t *testing.T) (*gorm.DB, *AlertService, *int) {
	t.Helper()
	database := setupAlertServiceDatabase(t)
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})
	requests := new(int)
	svc := NewAlertService(database, utils.NewLogger(), &config.Config{})
	svc.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		*requests++
		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Header: make(http.Header)}, nil
	})
	return database, svc, requests
}

func linkedService(t *testing.T, database *gorm.DB, id, name string) {
	t.Helper()
	now := time.Now().UTC()
	if err := database.Create(&db.Service{ID: id, Name: name, Slug: id, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
}

// incidentOn opens an incident on a new monitor of the given type, linked to serviceID ("" for none).
func incidentOn(t *testing.T, database *gorm.DB, incidentID, monitorType, serviceID string) {
	t.Helper()
	createTestIncident(t, database, incidentID)
	var sid *string
	if serviceID != "" {
		sid = &serviceID
	}
	now := time.Now().UTC()
	if err := database.Create(&db.Monitor{ID: "monitor-" + incidentID, NodeID: "node-1", Name: incidentID, Type: monitorType, Lifecycle: "active",
		Health: "down", ComputedHealth: "down", IncidentState: "down", ServiceID: sid, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
}

func deliveriesByIncident(t *testing.T, database *gorm.DB) map[string]db.AlertDelivery {
	t.Helper()
	var all []db.AlertDelivery
	if err := database.Order("created_at ASC").Find(&all).Error; err != nil {
		t.Fatal(err)
	}
	out := map[string]db.AlertDelivery{}
	for _, d := range all {
		out[d.IncidentID] = d
	}
	return out
}

func TestAlertGroupingJoinsACheckFailureAndALogBurstOfOneService(t *testing.T) {
	database, svc, requests := groupingFixture(t)
	linkedService(t, database, "svc-checkout", "Checkout")
	// The check failure: an HTTP monitor linked to the service.
	incidentOn(t, database, "inc-check", "http", "svc-checkout")
	// The log burst: the log rule's own monitor, which has no link of its own; its app is linked.
	incidentOn(t, database, "inc-logs", "log-alert", "")
	now := time.Now().UTC()
	sid := "svc-checkout"
	database.Create(&db.App{ID: "app-1", Name: "checkout-api", Slug: "checkout-api", ServiceID: &sid, CreatedAt: now, UpdatedAt: now})
	database.Create(&db.LogAlertRule{ID: "rule-1", Name: "errors", AppID: "app-1", Mode: "threshold", WindowSeconds: 300, Enabled: true, MonitorID: "monitor-inc-logs", CreatedAt: now, UpdatedAt: now})

	if err := queueAndDispatch(svc, "inc-check", db.AlertEventIncidentOpened); err != nil {
		t.Fatal(err)
	}
	if err := queueAndDispatch(svc, "inc-logs", db.AlertEventIncidentOpened); err != nil {
		t.Fatal(err)
	}

	got := deliveriesByIncident(t, database)
	if got["inc-check"].Status != "sent" || got["inc-logs"].Status != "suppressed" || got["inc-logs"].Error != "alert grouped into active alert group" {
		t.Fatalf("deliveries = check %+v, logs %+v; want one sent and one grouped", got["inc-check"], got["inc-logs"])
	}
	if got["inc-check"].AlertGroupID == "" || got["inc-check"].AlertGroupID != got["inc-logs"].AlertGroupID {
		t.Fatalf("group ids differ: %q vs %q", got["inc-check"].AlertGroupID, got["inc-logs"].AlertGroupID)
	}
	if *requests != 1 {
		t.Fatalf("webhook requests = %d, want one page for the service", *requests)
	}
	var group db.AlertGroup
	database.Where("id = ?", got["inc-check"].AlertGroupID).First(&group)
	if group.GroupKey != "service:svc-checkout" || group.IncidentCount != 2 || group.Summary != "2 incident(s) on service Checkout" {
		t.Fatalf("group = %+v", group)
	}
}

func TestAlertGroupingKeepsServicesAndUnlinkedMonitorsApart(t *testing.T) {
	database, svc, requests := groupingFixture(t)
	linkedService(t, database, "svc-a", "Checkout")
	linkedService(t, database, "svc-b", "Billing")
	incidentOn(t, database, "inc-a", "http", "svc-a")
	incidentOn(t, database, "inc-b", "tcp", "svc-b")
	for _, id := range []string{"inc-a", "inc-b"} {
		if err := queueAndDispatch(svc, id, db.AlertEventIncidentOpened); err != nil {
			t.Fatal(err)
		}
	}
	got := deliveriesByIncident(t, database)
	if got["inc-a"].Status != "sent" || got["inc-b"].Status != "sent" || got["inc-a"].AlertGroupID == got["inc-b"].AlertGroupID || *requests != 2 {
		t.Fatalf("two services must page separately: %+v / %+v requests=%d", got["inc-a"], got["inc-b"], *requests)
	}
}

func TestAlertGroupingServiceWindowIsBoundedByOverlap(t *testing.T) {
	database, svc, _ := groupingFixture(t)
	linkedService(t, database, "svc-a", "Checkout")
	incidentOn(t, database, "inc-old", "http", "svc-a")
	if err := queueAndDispatch(svc, "inc-old", db.AlertEventIncidentOpened); err != nil {
		t.Fatal(err)
	}
	old := deliveriesByIncident(t, database)["inc-old"]
	// The first outage ended long ago, but its group was never closed.
	database.Model(&db.Incident{}).Where("id = ?", "inc-old").Update("status", "resolved")
	database.Model(&db.AlertGroup{}).Where("id = ?", old.AlertGroupID).Update("last_event_at", time.Now().UTC().Add(-2*time.Hour))

	incidentOn(t, database, "inc-new", "http", "svc-a")
	if err := queueAndDispatch(svc, "inc-new", db.AlertEventIncidentOpened); err != nil {
		t.Fatal(err)
	}
	fresh := deliveriesByIncident(t, database)["inc-new"]
	if fresh.Status != "sent" || fresh.AlertGroupID == old.AlertGroupID {
		t.Fatalf("a stale group with no active incident must not swallow a new outage: %+v", fresh)
	}

	// An old group that still has an active incident does take a new one, however long it has run.
	database.Model(&db.AlertGroup{}).Where("id = ?", fresh.AlertGroupID).Update("last_event_at", time.Now().UTC().Add(-3*time.Hour))
	incidentOn(t, database, "inc-third", "tcp", "svc-a")
	if err := queueAndDispatch(svc, "inc-third", db.AlertEventIncidentOpened); err != nil {
		t.Fatal(err)
	}
	third := deliveriesByIncident(t, database)["inc-third"]
	if third.Status != "suppressed" || third.AlertGroupID != fresh.AlertGroupID {
		t.Fatalf("an overlapping incident must join the running group: %+v", third)
	}
}

func TestIncidentServiceOfIgnoresAStaleLink(t *testing.T) {
	database, _, _ := groupingFixture(t)
	incidentOn(t, database, "inc-x", "http", "svc-gone")
	if id, name, err := incidentServiceOf(database, "monitor-inc-x"); err != nil || id != "" || name != "" {
		t.Fatalf("stale link = %q %q %v, want none", id, name, err)
	}
	if id, _, err := incidentServiceOf(database, ""); err != nil || id != "" {
		t.Fatalf("no monitor = %q %v", id, err)
	}
}
