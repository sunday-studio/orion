package service

import (
	"encoding/json"
	"fmt"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"testing"
	"time"
)

// Chaos 3: a Worker killed mid-check leaves a lease behind; once it expires another
// Worker re-claims the check exactly once and the dead Worker cannot complete it.
func TestChaosWorkerKilledMidCheckLeaseExpiresAndIsReclaimedOnce(t *testing.T) {
	database := testkit.NewDB(t)
	a, _ := testkit.TwoAgents(t, database, time.Now().UTC())
	database.Create(&db.CoreMonitorConfig{MonitorID: a.Monitor.ID, Kind: "http", ConfigJSON: "{}", SecretRefJSON: "{}", IntervalSeconds: 60, TimeoutSeconds: 10, NextRunAt: time.Now().UTC().Add(-time.Minute)})
	scheduler := NewCoreMonitorSchedulerService(database, utils.NewLogger())
	start := time.Now().UTC()

	claimedByA, err := scheduler.ClaimDueCoreMonitorConfigs(ClaimDueCoreMonitorConfigsRequest{LeaseOwner: "worker-a", Limit: 5, LeaseDuration: 2 * time.Minute, Now: start})
	if err != nil || len(claimedByA) != 1 {
		t.Fatalf("worker A claimed %d (err %v), want 1", len(claimedByA), err)
	}
	// Worker A is killed here. While its lease is live nobody else may take the check.
	if during, _ := scheduler.ClaimDueCoreMonitorConfigs(ClaimDueCoreMonitorConfigsRequest{LeaseOwner: "worker-b", Limit: 5, LeaseDuration: 2 * time.Minute, Now: start.Add(time.Minute)}); len(during) != 0 {
		t.Fatalf("worker B claimed %d during A's lease, want 0", len(during))
	}
	// After expiry B takes it over, once, even if two B ticks race.
	var total int
	for range 3 {
		got, err := scheduler.ClaimDueCoreMonitorConfigs(ClaimDueCoreMonitorConfigsRequest{LeaseOwner: "worker-b", Limit: 5, LeaseDuration: 2 * time.Minute, Now: start.Add(3 * time.Minute)})
		if err != nil {
			t.Fatal(err)
		}
		total += len(got)
	}
	if total != 1 {
		t.Fatalf("worker B claimed the abandoned check %d times, want exactly once", total)
	}
	// A "comes back from the dead" and tries to complete: rejected because it lost the lease.
	if _, err := scheduler.CompleteCoreMonitorCheck(CompleteCoreMonitorCheckRequest{MonitorID: a.Monitor.ID, LeaseOwner: "worker-a", FinishedAt: start.Add(4 * time.Minute), Success: true}); err == nil {
		t.Fatal("a Worker that lost its lease completed the check")
	}
}

// Chaos 6: an agent clock ten minutes off, either way, does not change health.
func TestChaosAgentClockSkewDoesNotChangeHealth(t *testing.T) {
	for name, skew := range map[string]time.Duration{"clock behind": -10 * time.Minute, "clock ahead": 10 * time.Minute} {
		t.Run(name, func(t *testing.T) {
			database := testkit.NewDB(t)
			now := time.Now().UTC()
			a, _ := testkit.TwoAgents(t, database, now)
			database.Where("monitor_id = ?", a.Monitor.ID).Delete(&db.MonitorReport{})
			// Fresh on Core's clock, but the agent stamped it ten minutes off.
			database.Create(&db.MonitorReport{ID: "skewed", MonitorID: a.Monitor.ID, Payload: "{}", CollectedAt: now.Add(skew).Format(time.RFC3339), Health: "up", CreatedAt: now})

			health, err := NewHealthService(database, utils.NewLogger()).ComputeMonitorHealth(a.Monitor.ID, DefaultHealthConfig())
			if err != nil || health != "up" {
				t.Fatalf("health = %q err = %v, want up: the report arrived just now", health, err)
			}
			stale, _ := NewHealthService(database, utils.NewLogger()).DetectStaleMonitorsForAgent(a.Agent.ID, DefaultHealthConfig())
			if len(stale) != 0 {
				t.Fatalf("skewed but fresh report flagged stale: %v", stale)
			}
		})
	}
}

// Chaos 7: the webhook receiver is down for about an hour, then returns. The whole
// backlog drains, in creation order, with each incident delivered exactly once.
func TestChaosWebhookReceiverDownThenUpDrainsBacklogInOrderWithoutDuplicates(t *testing.T) {
	database := testkit.NewDB(t)
	hook := testkit.NewWebhook(t)
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops", Type: "webhook", Enabled: true, WebhookURL: hook.URL()})
	svc := NewAlertService(database, utils.NewLogger(), &config.Config{AlertAllowPrivateWebhooks: true})

	const backlog = 25
	base := time.Now().UTC().Add(-2 * time.Hour)
	var order []string
	for i := 0; i < backlog; i++ {
		id := fmt.Sprintf("incident-%02d", i)
		order = append(order, id)
		createTestIncident(t, database, id)
		// Distinct agents, so alert grouping does not fold the backlog into one page.
		database.Model(&db.Incident{}).Where("id = ?", id).Update("agent_id", "agent-"+id)
		if err := svc.QueueIncidentNotifications(id, "incident_opened"); err != nil {
			t.Fatal(err)
		}
		database.Model(&db.AlertDelivery{}).Where("incident_id = ?", id).Update("created_at", base.Add(time.Duration(i)*time.Second))
	}

	// The receiver is down for the first seven rounds of retries (about an hour of
	// real backoff: 30s+1m+2m+5m+10m+15m+30m), then comes back.
	hook.FailFirst(backlog * (defaultAlertDeliveryMaxAttempts - 1))
	for round := 1; round <= defaultAlertDeliveryMaxAttempts; round++ {
		if _, err := svc.ProcessDueDeliveries(backlog * 2); err != nil {
			t.Fatal(err)
		}
		makeAllDeliveriesDue(svc) // advance past the backoff
	}

	var deliveries []db.AlertDelivery
	database.Order("created_at ASC").Find(&deliveries)
	if len(deliveries) != backlog {
		t.Fatalf("deliveries = %d, want %d", len(deliveries), backlog)
	}
	for _, d := range deliveries {
		if d.Status != "sent" || d.AttemptCount != defaultAlertDeliveryMaxAttempts {
			t.Fatalf("delivery %s status=%s attempts=%d, want sent on the final attempt", d.IncidentID, d.Status, d.AttemptCount)
		}
	}

	// The receiver saw each incident exactly once on success, in creation order.
	var successful []string
	seen := map[string]int{}
	for i, raw := range hook.Bodies() {
		if i < backlog*(defaultAlertDeliveryMaxAttempts-1) {
			continue // failed attempts
		}
		var payload struct {
			Incident struct {
				ID string `json:"id"`
			} `json:"incident"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
		successful = append(successful, payload.Incident.ID)
		seen[payload.Incident.ID]++
	}
	if len(successful) != backlog {
		t.Fatalf("successful sends = %d, want %d", len(successful), backlog)
	}
	for i, id := range successful {
		if id != order[i] || seen[id] != 1 {
			t.Fatalf("send %d was %s (seen %d times), want %s once: backlog must drain in order without duplicates", i, id, seen[id], order[i])
		}
	}
}
