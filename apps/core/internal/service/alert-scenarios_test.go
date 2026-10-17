package service

import (
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"testing"
	"time"
)

func scenarioAlerting(t *testing.T) (*AlertService, *testkit.Webhook) {
	t.Helper()
	database := testkit.NewDB(t)
	createTestIncident(t, database, "incident-1")
	hook := testkit.NewWebhook(t)
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops", Type: "webhook", Enabled: true, WebhookURL: hook.URL()})
	svc := NewAlertService(database, utils.NewLogger(), &config.Config{AlertAllowPrivateWebhooks: true})
	return svc, hook
}

func makeAllDeliveriesDue(svc *AlertService) {
	past := time.Now().UTC().Add(-time.Minute)
	svc.db.Model(&db.AlertDelivery{}).Where("status = ?", "failed").Update("next_attempt_at", past)
}

// A1: webhook returns 500 once then 200 -> failed then sent, exactly two attempts.
func TestScenarioA1WebhookFailsOnceThenSucceeds(t *testing.T) {
	svc, hook := scenarioAlerting(t)
	hook.FailFirst(1)
	if err := queueAndDispatch(svc, "incident-1", "incident_opened"); err != nil {
		t.Fatal(err)
	}
	makeAllDeliveriesDue(svc)
	if _, err := svc.ProcessDueDeliveries(10); err != nil {
		t.Fatal(err)
	}
	var d db.AlertDelivery
	svc.db.Preload("Attempts").First(&d)
	if d.Status != "sent" || len(d.Attempts) != 2 || hook.Calls() != 2 {
		t.Fatalf("delivery status=%s attempts=%d calls=%d, want sent/2/2", d.Status, len(d.Attempts), hook.Calls())
	}
}

// A2: always failing -> retried up to max attempts, then no further sends.
func TestScenarioA2WebhookAlwaysFailsStopsAtMaxAttempts(t *testing.T) {
	svc, hook := scenarioAlerting(t)
	hook.AlwaysFail()
	if err := svc.QueueIncidentNotifications("incident-1", "incident_opened"); err != nil {
		t.Fatal(err)
	}
	for range defaultAlertDeliveryMaxAttempts + 3 {
		if _, err := svc.ProcessDueDeliveries(10); err != nil {
			t.Fatal(err)
		}
		makeAllDeliveriesDue(svc)
	}
	var d db.AlertDelivery
	svc.db.First(&d)
	if d.Status != "failed" || d.AttemptCount != defaultAlertDeliveryMaxAttempts || hook.Calls() != defaultAlertDeliveryMaxAttempts {
		t.Fatalf("status=%s attempts=%d calls=%d, want failed after exactly %d", d.Status, d.AttemptCount, hook.Calls(), defaultAlertDeliveryMaxAttempts)
	}
}

// A3: a slow receiver must not slow the queueing path (ingestion).
func TestScenarioA3SlowWebhookDoesNotBlockQueueing(t *testing.T) {
	svc, hook := scenarioAlerting(t)
	hook.Delay(2 * time.Second)
	started := time.Now()
	if err := svc.QueueIncidentNotifications("incident-1", "incident_opened"); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("queueing took %v with a slow webhook, want well under 1s", elapsed)
	}
	if hook.Calls() != 0 {
		t.Fatalf("webhook called %d times during queueing, want 0", hook.Calls())
	}
}

// An operator retry of an exhausted delivery is sent by the next dispatch.
func TestRetryDeliveryAfterExhaustionIsSentByDispatcher(t *testing.T) {
	svc, hook := scenarioAlerting(t)
	hook.FailFirst(defaultAlertDeliveryMaxAttempts) // every automatic attempt fails
	if err := svc.QueueIncidentNotifications("incident-1", "incident_opened"); err != nil {
		t.Fatal(err)
	}
	for range defaultAlertDeliveryMaxAttempts {
		svc.ProcessDueDeliveries(10)
		makeAllDeliveriesDue(svc)
	}
	var d db.AlertDelivery
	svc.db.First(&d)
	if d.Status != "failed" || d.AttemptCount != defaultAlertDeliveryMaxAttempts {
		t.Fatalf("setup: status=%s attempts=%d, want failed after %d", d.Status, d.AttemptCount, defaultAlertDeliveryMaxAttempts)
	}

	if _, err := svc.RetryDelivery(d.ID); err != nil {
		t.Fatalf("RetryDelivery() error = %v", err)
	}
	if _, err := svc.ProcessDueDeliveries(10); err != nil {
		t.Fatal(err)
	}
	svc.db.First(&d, "id = ?", d.ID)
	if d.Status != "sent" || d.AttemptCount != defaultAlertDeliveryMaxAttempts+1 {
		t.Fatalf("after operator retry status=%s attempts=%d, want sent on attempt %d", d.Status, d.AttemptCount, defaultAlertDeliveryMaxAttempts+1)
	}
}

// An operator retry must not steal a delivery that a dispatcher is sending right now.
func TestRetryDeliveryRefusesWhileAnAttemptIsInFlight(t *testing.T) {
	svc, _ := scenarioAlerting(t)
	if err := svc.QueueIncidentNotifications("incident-1", "incident_opened"); err != nil {
		t.Fatal(err)
	}
	var d db.AlertDelivery
	svc.db.First(&d)
	live := time.Now().UTC().Add(time.Minute)
	svc.db.Model(&db.AlertDelivery{}).Where("id = ?", d.ID).Updates(map[string]any{"status": "failed", "locked_by": "dispatcher-x", "locked_until": &live})

	if _, err := svc.RetryDelivery(d.ID); err != ErrAlertDeliveryNotRetryable {
		t.Fatalf("RetryDelivery() during an in-flight attempt = %v, want ErrAlertDeliveryNotRetryable", err)
	}
	svc.db.First(&d, "id = ?", d.ID)
	if d.LockedBy != "dispatcher-x" {
		t.Fatal("the retry cleared another dispatcher's lease")
	}
}
