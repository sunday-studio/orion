package service

import (
	"fmt"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logstore"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"testing"
	"time"

	"gorm.io/gorm"
)

type logAlertFixture struct {
	database *gorm.DB
	store    *logstore.Store
	svc      *LogAlertService
	hook     *testkit.Webhook
	app      db.App
	now      time.Time
}

func newLogAlertFixture(t *testing.T) *logAlertFixture {
	t.Helper()
	database := testkit.NewDB(t)
	store, err := logstore.OpenWith(testkit.NewDB(t), utils.NewLogger(), 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	hook := testkit.NewWebhook(t)
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops", Type: "webhook", Enabled: true, WebhookURL: hook.URL()})
	now := time.Now().UTC().Truncate(time.Second)
	app := db.App{ID: "app-1", Name: "checkout", Slug: "checkout", RetentionDays: 14, DebugSampleRate: 1, DisplayJSON: "{}", RedactionJSON: "{}", CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now}
	database.Create(&app)
	cfg := &config.Config{AlertAllowPrivateWebhooks: true, AlertRecoveryNotifications: true}
	return &logAlertFixture{database: database, store: store, svc: NewLogAlertService(database, utils.NewLogger(), cfg, store), hook: hook, app: app, now: now}
}

func (f *logAlertFixture) addLogs(t *testing.T, n int, severity string, number int, event string, at time.Time) {
	t.Helper()
	var batch []logstore.Entry
	for i := 0; i < n; i++ {
		ts := at.Add(time.Duration(i) * time.Millisecond)
		batch = append(batch, logstore.Entry{ID: logstore.NewEntryID(ts), AppID: f.app.ID, Timestamp: ts, IngestedAt: ts, Severity: severity, SeverityNumber: number, Event: event, Body: fmt.Sprintf("entry %d", i), Size: 50})
	}
	if err := f.store.Enqueue(batch); err != nil {
		t.Fatal(err)
	}
	f.store.Flush(5 * time.Second)
}

func (f *logAlertFixture) rule(t *testing.T, r db.LogAlertRule) *db.LogAlertRule {
	t.Helper()
	r.ID, r.AppID, r.Enabled, r.State = "rule-1", f.app.ID, true, "ok"
	if r.Mode == "" {
		r.Mode = LogAlertModeThreshold
	}
	if r.WindowSeconds == 0 {
		r.WindowSeconds = 300
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = f.now.Add(-time.Hour)
	}
	r.UpdatedAt = f.now
	if err := f.database.Create(&r).Error; err != nil {
		t.Fatal(err)
	}
	return &r
}

func (f *logAlertFixture) incidentsFor(rule *db.LogAlertRule) []db.Incident {
	var incidents []db.Incident
	f.database.Where("monitor_id = ?", logAlertMonitorID+rule.ID).Order("opened_at ASC").Find(&incidents)
	return incidents
}

func (f *logAlertFixture) deliveries(event string) int64 {
	var n int64
	f.database.Model(&db.AlertDelivery{}).Where("event_type = ?", event).Count(&n)
	return n
}

// Plan gate: an error burst pages once, then resolves when the burst ends.
func TestLogAlertFiresOnceOnAnErrorBurstAndResolves(t *testing.T) {
	f := newLogAlertFixture(t)
	rule := f.rule(t, db.LogAlertRule{Name: "Checkout errors", Severity: ">=error", Threshold: 20})
	f.addLogs(t, 500, "info", 9, "", f.now.Add(-time.Minute)) // noise that must not count

	if changed, err := f.svc.Evaluate(rule, f.now); err != nil || changed {
		t.Fatalf("quiet evaluate = %v, %v, want no state change", changed, err)
	}
	f.addLogs(t, 30, "error", 17, "payment.failed", f.now.Add(-30*time.Second))
	changed, err := f.svc.Evaluate(rule, f.now)
	if err != nil || !changed || rule.State != "firing" || rule.LastCount != 30 {
		t.Fatalf("burst evaluate = changed %v err %v rule %+v, want firing with 30 matches", changed, err, rule)
	}
	incidents := f.incidentsFor(rule)
	if len(incidents) != 1 || incidents[0].Status != "open" || incidents[0].Title != "Core monitor down: Log alert: Checkout errors" {
		t.Fatalf("incidents = %+v, want one open incident", incidents)
	}

	// The burst continues for several evaluations: still one incident, one page.
	for i := 1; i <= 5; i++ {
		f.addLogs(t, 5, "error", 17, "payment.failed", f.now.Add(time.Duration(i)*20*time.Second))
		if _, err := f.svc.Evaluate(rule, f.now.Add(time.Duration(i)*30*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.incidentsFor(rule)) != 1 || f.deliveries("incident_opened") != 1 {
		t.Fatalf("incidents=%d opened deliveries=%d, want exactly one page for the whole burst", len(f.incidentsFor(rule)), f.deliveries("incident_opened"))
	}

	// The burst ends and its entries age out of the window: the incident resolves.
	later := f.now.Add(20 * time.Minute)
	changed, err = f.svc.Evaluate(rule, later)
	if err != nil || !changed || rule.State != "ok" {
		t.Fatalf("recovery evaluate = %v, %v, state %s", changed, err, rule.State)
	}
	if got := f.incidentsFor(rule)[0].Status; got != "resolved" {
		t.Fatalf("incident status = %q, want resolved", got)
	}
	if f.deliveries("incident_resolved") != 1 {
		t.Fatalf("resolved deliveries = %d, want 1", f.deliveries("incident_resolved"))
	}
}

func TestLogAlertThresholdIsStrictlyGreaterThan(t *testing.T) {
	f := newLogAlertFixture(t)
	rule := f.rule(t, db.LogAlertRule{Name: "edge", Severity: "error", Threshold: 10})
	f.addLogs(t, 10, "error", 17, "", f.now.Add(-time.Minute))
	if _, err := f.svc.Evaluate(rule, f.now); err != nil || rule.State != "ok" {
		t.Fatalf("exactly at the threshold: state %s err %v, want ok", rule.State, err)
	}
	f.addLogs(t, 1, "error", 17, "", f.now.Add(-30*time.Second))
	if _, err := f.svc.Evaluate(rule, f.now); err != nil || rule.State != "firing" {
		t.Fatalf("one over the threshold: state %s err %v, want firing", rule.State, err)
	}
}

func TestLogAlertAbsenceFiresWhenAnExpectedEventStopsAndRecovers(t *testing.T) {
	f := newLogAlertFixture(t)
	fresh := f.rule(t, db.LogAlertRule{Name: "nightly export", Mode: LogAlertModeAbsence, Event: "export.finished", WindowSeconds: 3600, CreatedAt: f.now.Add(-10 * time.Minute)})
	if _, err := f.svc.Evaluate(fresh, f.now); err != nil || fresh.State != "ok" {
		t.Fatalf("brand-new absence rule state = %s err %v, want ok: it gets one full window first", fresh.State, err)
	}

	f.database.Model(&db.LogAlertRule{}).Where("id = ?", fresh.ID).Update("created_at", f.now.Add(-3*time.Hour))
	fresh.CreatedAt = f.now.Add(-3 * time.Hour)
	if changed, err := f.svc.Evaluate(fresh, f.now); err != nil || !changed || fresh.State != "firing" {
		t.Fatalf("silent for a window: changed %v state %s err %v, want firing", changed, fresh.State, err)
	}
	f.addLogs(t, 1, "info", 9, "export.finished", f.now.Add(5*time.Minute))
	if changed, err := f.svc.Evaluate(fresh, f.now.Add(10*time.Minute)); err != nil || !changed || fresh.State != "ok" {
		t.Fatalf("event seen again: changed %v state %s err %v, want ok", changed, fresh.State, err)
	}
	if got := f.incidentsFor(fresh)[0].Status; got != "resolved" {
		t.Fatalf("incident status = %q, want resolved", got)
	}
}

func TestLogAlertRetireResolvesTheOpenIncident(t *testing.T) {
	f := newLogAlertFixture(t)
	rule := f.rule(t, db.LogAlertRule{Name: "temp", Threshold: 0})
	f.addLogs(t, 3, "error", 17, "", f.now.Add(-time.Minute))
	f.svc.Evaluate(rule, f.now)
	if rule.State != "firing" {
		t.Fatal("setup: rule should be firing")
	}
	if err := f.svc.Retire(rule, f.now); err != nil {
		t.Fatal(err)
	}
	if got := f.incidentsFor(rule)[0].Status; got != "resolved" {
		t.Fatalf("incident after retire = %q, want resolved", got)
	}
	var monitor db.Monitor
	f.database.First(&monitor, "id = ?", logAlertMonitorID+rule.ID)
	if monitor.Lifecycle != "disabled" {
		t.Fatalf("monitor lifecycle = %q, want parked", monitor.Lifecycle)
	}
}

func TestEvaluateAllSkipsDisabledRulesAndKeepsGoingAfterAnError(t *testing.T) {
	f := newLogAlertFixture(t)
	f.rule(t, db.LogAlertRule{Name: "on", Threshold: 0})
	off := db.LogAlertRule{ID: "rule-off", Name: "off", AppID: f.app.ID, Mode: LogAlertModeThreshold, WindowSeconds: 300, State: "ok", CreatedAt: f.now, UpdatedAt: f.now}
	f.database.Create(&off)
	f.database.Model(&db.LogAlertRule{}).Where("id = ?", "rule-off").Update("enabled", false)
	f.addLogs(t, 2, "error", 17, "", f.now.Add(-time.Second))

	changed, err := f.svc.EvaluateAll(f.now)
	if err != nil || changed != 1 {
		t.Fatalf("EvaluateAll() = %d, %v, want only the enabled rule to change", changed, err)
	}
	var untouched db.LogAlertRule
	f.database.First(&untouched, "id = ?", "rule-off")
	if untouched.LastEvaluatedAt != nil {
		t.Fatal("a disabled rule was evaluated")
	}
}

// Review P2: rules share the Core owner node, so grouping by node would swallow the second page.
func TestTwoFiringRulesPageSeparately(t *testing.T) {
	f := newLogAlertFixture(t)
	second := db.App{ID: "app-2", Name: "billing", Slug: "billing", RetentionDays: 14, DebugSampleRate: 1, DisplayJSON: "{}", RedactionJSON: "{}", CreatedAt: f.now, UpdatedAt: f.now}
	f.database.Create(&second)
	one := f.rule(t, db.LogAlertRule{Name: "checkout errors", Severity: ">=error", Threshold: 1})
	two := db.LogAlertRule{ID: "rule-2", Name: "billing errors", AppID: second.ID, Mode: LogAlertModeThreshold, Severity: ">=error", Threshold: 1, WindowSeconds: 300, Enabled: true, State: "ok", CreatedAt: f.now.Add(-time.Hour), UpdatedAt: f.now}
	f.database.Create(&two)
	f.addLogs(t, 5, "error", 17, "", f.now.Add(-time.Minute))
	f.app = second
	f.addLogs(t, 5, "error", 17, "", f.now.Add(-time.Minute))

	if _, err := f.svc.Evaluate(one, f.now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Evaluate(&two, f.now); err != nil {
		t.Fatal(err)
	}
	alerts := NewAlertService(f.database, utils.NewLogger(), &config.Config{AlertAllowPrivateWebhooks: true})
	if _, err := alerts.ProcessDueDeliveries(20); err != nil {
		t.Fatal(err)
	}
	if f.hook.Calls() != 2 {
		t.Fatalf("pages sent = %d, want one per firing rule (grouping must not suppress the second)", f.hook.Calls())
	}
	if f.deliveries("incident_opened") != 2 {
		t.Fatalf("opened deliveries = %d, want 2", f.deliveries("incident_opened"))
	}
}

// Review P3: a stale rule list must not revive a monitor that Retire just parked.
func TestEvaluateIgnoresARuleDisabledSinceTheListWasLoaded(t *testing.T) {
	f := newLogAlertFixture(t)
	rule := f.rule(t, db.LogAlertRule{Name: "stale view", Threshold: 0})
	f.database.Model(&db.LogAlertRule{}).Where("id = ?", rule.ID).Update("enabled", false) // disabled after EvaluateAll loaded it
	f.addLogs(t, 3, "error", 17, "", f.now.Add(-time.Second))
	if changed, err := f.svc.Evaluate(rule, f.now); err != nil || changed {
		t.Fatalf("Evaluate() = %v, %v, want a no-op for a disabled rule", changed, err)
	}
	var monitors int64
	f.database.Model(&db.Monitor{}).Where("id = ?", logAlertMonitorID+rule.ID).Count(&monitors)
	if monitors != 0 || len(f.incidentsFor(rule)) != 0 {
		t.Fatal("a disabled rule created a monitor or an incident")
	}
	f.database.Delete(&db.LogAlertRule{}, "id = ?", rule.ID)
	if changed, err := f.svc.Evaluate(rule, f.now); err != nil || changed {
		t.Fatalf("Evaluate() of a deleted rule = %v, %v, want a no-op", changed, err)
	}
}
