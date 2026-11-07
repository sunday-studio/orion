package service

import (
	"orion/core/internal/db"
	"reflect"
	"testing"
)

func TestIncidentLogSourceResolution(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	checkout, _ := f.catalog.Create(ServiceInput{Name: "Checkout"})
	empty, _ := f.catalog.Create(ServiceInput{Name: "Empty"})
	f.app(t, "app-api", "checkout-api")
	f.app(t, "app-worker", "checkout-worker")
	f.app(t, "app-rule", "rule-app")
	for _, id := range []string{"app-api", "app-worker"} {
		if _, err := f.catalog.Link(checkout.ID, ServiceLinkRef{Kind: "app", ID: id}); err != nil {
			t.Fatal(err)
		}
	}

	f.monitor(t, "mon-linked", "checkout health", nil)
	f.monitor(t, "mon-linked-rule", "Log alert: errors", func(m *db.Monitor) { m.Type = "log-alert" })
	f.monitor(t, "mon-empty-service", "empty health", nil)
	f.monitor(t, "mon-empty-service-rule", "empty rule monitor", nil)
	f.monitor(t, "mon-rule-only", "Log alert: burst", func(m *db.Monitor) { m.Type = "log-alert" })
	f.monitor(t, "mon-unlinked", "plain", nil)
	for monitor, svc := range map[string]string{"mon-linked": checkout.ID, "mon-linked-rule": checkout.ID, "mon-empty-service": empty.ID, "mon-empty-service-rule": empty.ID} {
		if _, err := f.catalog.Link(svc, ServiceLinkRef{Kind: "monitor", ID: monitor}); err != nil {
			t.Fatal(err)
		}
	}
	rule := func(id, appID, monitorID string) {
		if err := f.database.Create(&db.LogAlertRule{ID: id, Name: id, AppID: appID, Mode: "threshold", WindowSeconds: 300, Enabled: true, State: "ok", MonitorID: monitorID, CreatedAt: f.now, UpdatedAt: f.now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	rule("r-linked", "app-rule", "mon-linked-rule")
	rule("r-empty", "app-rule", "mon-empty-service-rule")
	rule("r-only", "app-rule", "mon-rule-only")
	rule("r-orphan", "app-rule", "mon-deleted-elsewhere")

	tests := []struct {
		name      string
		monitorID string
		want      IncidentLogSource
	}{
		{"linked monitor uses all of the service's apps", "mon-linked", IncidentLogSource{Via: "service", ServiceID: checkout.ID, ServiceName: "Checkout", AppIDs: []string{"app-api", "app-worker"}}},
		{"the service wins over the monitor's own log rule", "mon-linked-rule", IncidentLogSource{Via: "service", ServiceID: checkout.ID, ServiceName: "Checkout", AppIDs: []string{"app-api", "app-worker"}}},
		{"service without apps falls back to the rule and keeps the service", "mon-empty-service-rule", IncidentLogSource{Via: "log_rule", ServiceID: empty.ID, ServiceName: "Empty", AppIDs: []string{"app-rule"}}},
		{"service without apps and without a rule has no logs but names the service", "mon-empty-service", IncidentLogSource{Via: "none", ServiceID: empty.ID, ServiceName: "Empty"}},
		{"unlinked monitor with a rule uses the rule's app", "mon-rule-only", IncidentLogSource{Via: "log_rule", AppIDs: []string{"app-rule"}}},
		{"rule for a monitor row that is gone still resolves", "mon-deleted-elsewhere", IncidentLogSource{Via: "log_rule", AppIDs: []string{"app-rule"}}},
		{"unlinked monitor without a rule", "mon-unlinked", IncidentLogSource{Via: "none"}},
		{"unknown monitor", "mon-unknown", IncidentLogSource{Via: "none"}},
		{"incident without a monitor", "", IncidentLogSource{Via: "none"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := f.catalog.IncidentLogSource(tc.monitorID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("IncidentLogSource(%q) = %+v, want %+v", tc.monitorID, got, tc.want)
			}
		})
	}
}

func TestIncidentLogSourceFollowsUnlinking(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	svc, _ := f.catalog.Create(ServiceInput{Name: "Checkout"})
	f.app(t, "app-1", "checkout")
	f.monitor(t, "mon-1", "checkout health", nil)
	for _, ref := range []ServiceLinkRef{{Kind: "app", ID: "app-1"}, {Kind: "monitor", ID: "mon-1"}} {
		if _, err := f.catalog.Link(svc.ID, ref); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := f.catalog.IncidentLogSource("mon-1"); got.Via != "service" {
		t.Fatalf("linked: %+v", got)
	}
	if err := f.catalog.Unlink(svc.ID, ServiceLinkRef{Kind: "app", ID: "app-1"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.catalog.IncidentLogSource("mon-1"); got.Via != "none" || got.ServiceID != svc.ID {
		t.Fatalf("app unlinked: %+v", got)
	}
	if _, err := f.catalog.Delete(svc.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.catalog.IncidentLogSource("mon-1"); got.Via != "none" || got.ServiceID != "" {
		t.Fatalf("service deleted: %+v", got)
	}
}
