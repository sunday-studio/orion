package service

import (
	"errors"
	"orion/core/internal/db"
	"testing"
	"time"
)

type fakeServiceNames struct {
	names map[string][]string
	err   error
}

func (f fakeServiceNames) RecentServiceNames(time.Time, int) (map[string][]string, error) {
	return f.names, f.err
}

func (f *catalogFixture) coreMonitor(t *testing.T, id, name, configJSON string) {
	t.Helper()
	f.monitor(t, id, name, func(m *db.Monitor) { m.Type = "http" })
	if err := f.database.Create(&db.CoreMonitorConfig{MonitorID: id, Kind: "http", ConfigJSON: configJSON, NextRunAt: f.now, CreatedAt: f.now, UpdatedAt: f.now}).Error; err != nil {
		t.Fatalf("create core monitor config: %v", err)
	}
}

// membersByID flattens a suggestion list to "serviceID|kind:ref" -> member for easy lookups.
func suggestionMembers(list []ServiceSuggestion) map[string]ServiceSuggestionMember {
	out := map[string]ServiceSuggestionMember{}
	for _, s := range list {
		for _, m := range s.Members {
			ref := m.ID
			if m.Kind == ServiceLinkResource {
				ref = m.NodeID + "/" + m.Key
			}
			out[s.Slug+"|"+m.Kind+":"+ref] = m
		}
	}
	return out
}

func TestServiceSuggestionsForExistingService(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	if _, err := f.catalog.Create(ServiceInput{Name: "Checkout"}); err != nil {
		t.Fatal(err)
	}
	f.app(t, "app-1", "checkout")
	f.monitor(t, "mon-name", "Checkout health", nil)
	f.monitor(t, "mon-exact", "checkout", nil)
	f.monitor(t, "mon-other", "billing health", nil)
	f.monitor(t, "mon-sys", "checkout watchdog", func(m *db.Monitor) { m.System = true })
	f.coreMonitor(t, "mon-host", "Storefront probe", `{"url":"https://checkout.example.com/health","method":"GET"}`)
	f.coreMonitor(t, "mon-ip", "Direct probe", `{"url":"http://10.0.0.5/health"}`)
	f.resource(t, "node-1", "unit:checkout", "unit", "checkout.service")
	f.resource(t, "node-1", "container:checkout-api-1", "container", "checkout-api-1")
	f.resource(t, "node-1", "path:/var/lib/checkout", "path", "checkout")
	f.resource(t, "node-1", "process:postgres", "process", "postgres")

	got, err := f.catalog.Suggest("")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ServiceID == "" || got[0].Slug != "checkout" {
		t.Fatalf("suggestions = %+v, want one for the existing checkout service", got)
	}
	members := suggestionMembers(got)

	want := map[string]struct{ matchedOn, against, confidence string }{
		"checkout|app:app-1":                                {"app_slug", "service_slug", "high"},
		"checkout|monitor:mon-exact":                        {"monitor_name", "service_name", "high"},
		"checkout|monitor:mon-name":                         {"monitor_name", "service_slug", "medium"},
		"checkout|monitor:mon-host":                         {"monitor_host", "service_name", "high"},
		"checkout|resource:node-1/unit:checkout":            {"resource_name", "service_name", "high"},
		"checkout|resource:node-1/container:checkout-api-1": {"resource_name", "service_slug", "medium"},
	}
	for ref, w := range want {
		m, ok := members[ref]
		if !ok {
			t.Errorf("missing suggestion %s", ref)
			continue
		}
		if m.MatchedOn != w.matchedOn || m.Confidence != w.confidence || m.Reason == "" {
			t.Errorf("%s = %+v, want matched_on %s, confidence %s and a reason", ref, m, w.matchedOn, w.confidence)
		}
	}
	for _, ref := range []string{"checkout|monitor:mon-other", "checkout|monitor:mon-sys", "checkout|monitor:mon-ip", "checkout|resource:node-1/process:postgres", "checkout|resource:node-1/path:/var/lib/checkout"} {
		if _, ok := members[ref]; ok {
			t.Errorf("%s was suggested but should not be", ref)
		}
	}
}

func TestServiceSuggestionsNeverLink(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	f.catalog.Create(ServiceInput{Name: "Checkout"})
	f.app(t, "app-1", "checkout")
	f.monitor(t, "mon-1", "checkout health", nil)
	f.resource(t, "node-1", "unit:checkout", "unit", "checkout.service")

	if got, err := f.catalog.Suggest(""); err != nil || len(got) == 0 {
		t.Fatalf("Suggest = %+v, %v", got, err)
	}
	for _, table := range []string{"apps", "monitors", "node_app_resources"} {
		var n int64
		f.database.Table(table).Where("service_id IS NOT NULL").Count(&n)
		if n != 0 {
			t.Fatalf("%s has %d linked rows after Suggest", table, n)
		}
	}
}

func TestServiceSuggestionsStartNewServiceFromOtelName(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	f.catalog = NewServiceCatalog(f.database, fakeServiceNames{names: map[string][]string{"app-pay": {"payments-svc"}}})
	f.app(t, "app-pay", "payments")
	f.app(t, "app-lonely", "lonely")
	f.monitor(t, "mon-pay", "payments-svc latency", nil)
	f.monitor(t, "mon-slug", "payments uptime", nil)
	f.resource(t, "node-1", "container:payments-svc", "container", "payments-svc")

	got, err := f.catalog.Suggest("")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("suggestions = %+v, want only the payments group (a lone app is not a suggestion)", got)
	}
	g := got[0]
	if g.ServiceID != "" || g.Name != "payments" || g.Slug != "payments" {
		t.Fatalf("group = %+v, want a new service named after the app", g)
	}
	members := suggestionMembers(got)
	if m := members["payments|app:app-pay"]; m.Reason == "" || m.Confidence != "high" {
		t.Errorf("anchor app = %+v", m)
	}
	if m := members["payments|monitor:mon-pay"]; m.Against != "otel_service_name" || m.Confidence != "medium" {
		t.Errorf("monitor matched on the OTel name = %+v", m)
	}
	if m := members["payments|monitor:mon-slug"]; m.Against != "app_slug" {
		t.Errorf("monitor matched on the slug = %+v", m)
	}
	if m := members["payments|resource:node-1/container:payments-svc"]; m.Against != "otel_service_name" || m.Confidence != "high" {
		t.Errorf("resource = %+v", m)
	}
}

func TestServiceSuggestionsSkipLinkedThings(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	checkout, _ := f.catalog.Create(ServiceInput{Name: "Checkout"})
	billing, _ := f.catalog.Create(ServiceInput{Name: "Billing"})
	f.app(t, "app-1", "checkout")
	f.monitor(t, "mon-linked-here", "checkout one", nil)
	f.monitor(t, "mon-linked-elsewhere", "checkout two", nil)
	f.monitor(t, "mon-free", "checkout three", nil)
	if _, err := f.catalog.Link(checkout.ID, ServiceLinkRef{Kind: "monitor", ID: "mon-linked-here"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.catalog.Link(billing.ID, ServiceLinkRef{Kind: "monitor", ID: "mon-linked-elsewhere"}); err != nil {
		t.Fatal(err)
	}

	got, err := f.catalog.Suggest(checkout.ID)
	if err != nil {
		t.Fatal(err)
	}
	members := suggestionMembers(got)
	if len(members) != 2 || members["checkout|app:app-1"].Kind == "" || members["checkout|monitor:mon-free"].Kind == "" {
		t.Fatalf("members = %+v, want only the unlinked app and monitor", members)
	}
}

func TestServiceSuggestionsPickTheBestServiceAndIgnoreShortKeys(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	f.catalog.Create(ServiceInput{Name: "Checkout"})
	f.catalog.Create(ServiceInput{Name: "Checkout API"})
	f.catalog.Create(ServiceInput{Name: "API"})
	f.monitor(t, "mon-1", "checkout-api health", nil)
	f.monitor(t, "mon-gw", "api-gateway health", nil)
	f.monitor(t, "mon-plain", "API", nil)

	got, err := f.catalog.Suggest("")
	if err != nil {
		t.Fatal(err)
	}
	members := suggestionMembers(got)
	if _, ok := members["checkout-api|monitor:mon-1"]; !ok {
		t.Errorf("checkout-api health should go to the longer matching key, got %+v", members)
	}
	if _, ok := members["checkout|monitor:mon-1"]; ok {
		t.Errorf("a monitor must be suggested for one service only")
	}
	if _, ok := members["api|monitor:mon-gw"]; ok {
		t.Errorf("a 3-letter key must not claim api-gateway by prefix")
	}
	if _, ok := members["api|monitor:mon-plain"]; !ok {
		t.Errorf("an exact match on a short key should still work")
	}
}

func TestServiceSuggestionsErrors(t *testing.T) {
	f := newCatalogFixture(t)
	_, err := f.catalog.Suggest("svc-missing")
	wantServiceError(t, err, ServiceErrNotFound, "Service not found")

	// A failing log store only costs the service.name rule.
	f.catalog = NewServiceCatalog(f.database, fakeServiceNames{err: errors.New("logs.db is busy")})
	f.app(t, "app-1", "checkout")
	f.monitor(t, "mon-1", "checkout health", nil)
	got, err := f.catalog.Suggest("")
	if err != nil || len(got) != 1 {
		t.Fatalf("Suggest = %+v, %v; want the slug-based suggestion despite the log error", got, err)
	}
}

func TestMonitorTargetHost(t *testing.T) {
	tests := []struct{ name, config, want string }{
		{"https url", `{"url":"https://Checkout.Example.com:8443/health?x=1"}`, "checkout.example.com"},
		{"tcp host", `{"host":"db.internal","port":5432}`, "db.internal"},
		{"ipv4", `{"host":"10.0.0.5"}`, ""},
		{"ipv6", `{"url":"http://[::1]:8080/"}`, ""},
		{"localhost", `{"url":"http://localhost/"}`, ""},
		{"not json", `nope`, ""},
		{"empty", `{}`, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := monitorTargetHost(tc.config); got != tc.want {
				t.Fatalf("monitorTargetHost(%s) = %q, want %q", tc.config, got, tc.want)
			}
		})
	}
}
