package service

import (
	"encoding/json"
	"errors"
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

type catalogFixture struct {
	database *gorm.DB
	catalog  *ServiceCatalog
	now      time.Time
}

func newCatalogFixture(t *testing.T) *catalogFixture {
	t.Helper()
	database := testkit.NewDB(t)
	return &catalogFixture{database: database, catalog: NewServiceCatalog(database, nil), now: time.Now().UTC().Truncate(time.Second)}
}

func (f *catalogFixture) node(t *testing.T, id string) {
	t.Helper()
	if err := f.database.Create(&db.Node{ID: id, MachineId: "machine-" + id, Name: id, OS: "linux", Arch: "amd64", Token: "tok-" + id, CreatedAt: f.now}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
}

func (f *catalogFixture) app(t *testing.T, id, slug string) db.App {
	t.Helper()
	app := db.App{ID: id, Name: slug, Slug: slug, RetentionDays: 14, DebugSampleRate: 1, DisplayJSON: "{}", RedactionJSON: "{}", CreatedAt: f.now, UpdatedAt: f.now}
	if err := f.database.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	return app
}

func (f *catalogFixture) monitor(t *testing.T, id, name string, mutate func(*db.Monitor)) db.Monitor {
	t.Helper()
	m := db.Monitor{ID: id, NodeID: "node-1", Name: name, Type: "http", Lifecycle: "active", Health: "up", ComputedHealth: "up", IncidentState: "unknown", CreatedAt: f.now, UpdatedAt: f.now}
	if mutate != nil {
		mutate(&m)
	}
	if err := f.database.Create(&m).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	return m
}

func (f *catalogFixture) resource(t *testing.T, nodeID, key, kind, name string) {
	t.Helper()
	if err := f.database.Create(&db.NodeAppResource{NodeID: nodeID, Key: key, Kind: kind, Name: name, UpdatedAt: f.now}).Error; err != nil {
		t.Fatalf("create resource: %v", err)
	}
}

func wantServiceError(t *testing.T, err error, kind ServiceErrorKind, contains string) {
	t.Helper()
	var se *ServiceError
	if !errors.As(err, &se) {
		t.Fatalf("error = %v, want a ServiceError of kind %s", err, kind)
	}
	if se.Kind != kind || !strings.Contains(se.Message, contains) {
		t.Fatalf("error = %q (%s), want kind %s containing %q", se.Message, se.Kind, kind, contains)
	}
}

func TestServiceCatalogCreateValidatesAndDerivesSlug(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	if _, err := f.catalog.Create(ServiceInput{Name: "Checkout", Environment: "production"}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		in       ServiceInput
		wantSlug string
		wantKind ServiceErrorKind
		wantMsg  string
	}{
		{name: "derives slug from name", in: ServiceInput{Name: "Billing API"}, wantSlug: "billing-api"},
		{name: "explicit slug", in: ServiceInput{Name: "Search", Slug: "search-svc"}, wantSlug: "search-svc"},
		{name: "taken slug adds the environment", in: ServiceInput{Name: "Checkout", Environment: "staging"}, wantSlug: "checkout-staging"},
		{name: "taken slug and environment adds a number", in: ServiceInput{Name: "Checkout", Environment: "staging", RunbookURL: "https://wiki.example.com/checkout"}, wantSlug: "checkout-2"},
		{name: "owner node", in: ServiceInput{Name: "Mail", NodeID: "node-1"}, wantSlug: "mail"},
		{name: "empty name", in: ServiceInput{Name: "  "}, wantKind: ServiceErrInvalid, wantMsg: "name is required"},
		{name: "name too long", in: ServiceInput{Name: strings.Repeat("a", 81)}, wantKind: ServiceErrInvalid, wantMsg: "at most 80"},
		{name: "bad slug", in: ServiceInput{Name: "X", Slug: "Not A Slug"}, wantKind: ServiceErrInvalid, wantMsg: "slug must be"},
		{name: "explicit slug already used", in: ServiceInput{Name: "Other", Slug: "checkout"}, wantKind: ServiceErrConflict, wantMsg: "slug already exists"},
		{name: "environment too long", in: ServiceInput{Name: "X", Environment: strings.Repeat("e", 41)}, wantKind: ServiceErrInvalid, wantMsg: "environment"},
		{name: "runbook must be http(s)", in: ServiceInput{Name: "X", RunbookURL: "ftp://example.com"}, wantKind: ServiceErrInvalid, wantMsg: "runbook_url"},
		{name: "unknown owner node", in: ServiceInput{Name: "X", NodeID: "node-missing"}, wantKind: ServiceErrInvalid, wantMsg: "node_id"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := f.catalog.Create(tc.in)
			if tc.wantKind != "" {
				wantServiceError(t, err, tc.wantKind, tc.wantMsg)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Slug != tc.wantSlug || got.ID == "" || got.Health != "unknown" {
				t.Fatalf("created %+v, want slug %q with an id and unknown health", got, tc.wantSlug)
			}
		})
	}
}

func TestServiceCatalogUpdate(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	a, _ := f.catalog.Create(ServiceInput{Name: "Checkout", NodeID: "node-1", Environment: "prod", RunbookURL: "https://wiki.example.com/x"})
	b, _ := f.catalog.Create(ServiceInput{Name: "Billing"})
	str := func(s string) *string { return &s }

	tests := []struct {
		name     string
		id       string
		patch    ServicePatch
		wantKind ServiceErrorKind
		check    func(t *testing.T, got ServiceSummary)
	}{
		{name: "rename keeps slug", id: a.ID, patch: ServicePatch{Name: str("Checkout API")}, check: func(t *testing.T, g ServiceSummary) {
			if g.Name != "Checkout API" || g.Slug != "checkout" {
				t.Fatalf("got %+v", g)
			}
		}},
		{name: "empty string clears optional fields", id: a.ID, patch: ServicePatch{NodeID: str(""), Environment: str(""), RunbookURL: str("")}, check: func(t *testing.T, g ServiceSummary) {
			if g.NodeID != "" || g.Environment != "" || g.RunbookURL != "" {
				t.Fatalf("got %+v", g)
			}
		}},
		{name: "slug taken by another service", id: a.ID, patch: ServicePatch{Slug: str(b.Slug)}, wantKind: ServiceErrConflict},
		{name: "own slug is fine", id: a.ID, patch: ServicePatch{Slug: str("checkout")}},
		{name: "bad slug", id: a.ID, patch: ServicePatch{Slug: str("-bad-")}, wantKind: ServiceErrInvalid},
		{name: "blank name", id: a.ID, patch: ServicePatch{Name: str("")}, wantKind: ServiceErrInvalid},
		{name: "unknown node", id: a.ID, patch: ServicePatch{NodeID: str("nope")}, wantKind: ServiceErrInvalid},
		{name: "missing service", id: "svc-missing", patch: ServicePatch{Name: str("x")}, wantKind: ServiceErrNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := f.catalog.Update(tc.id, tc.patch)
			if tc.wantKind != "" {
				wantServiceError(t, err, tc.wantKind, "")
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}
}

func TestServiceCatalogLinkAndUnlink(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	svc, _ := f.catalog.Create(ServiceInput{Name: "Checkout"})
	other, _ := f.catalog.Create(ServiceInput{Name: "Billing"})
	f.app(t, "app-1", "checkout-api")
	f.monitor(t, "mon-1", "checkout health", nil)
	f.monitor(t, "mon-sys", "worker liveness", func(m *db.Monitor) { m.System = true })
	f.monitor(t, "mon-del", "old", func(m *db.Monitor) { m.Lifecycle = "deleted" })
	f.resource(t, "node-1", "container:checkout-api", "container", "checkout-api")

	app := ServiceLinkRef{Kind: "app", ID: "app-1"}
	mon := ServiceLinkRef{Kind: "monitor", ID: "mon-1"}
	res := ServiceLinkRef{Kind: "resource", NodeID: "node-1", Key: "container:checkout-api"}

	t.Run("links each kind", func(t *testing.T) {
		for _, ref := range []ServiceLinkRef{app, mon, res} {
			got, err := f.catalog.Link(svc.ID, ref)
			if err != nil || got.ServiceID != svc.ID || got.PreviousServiceID != "" {
				t.Fatalf("Link(%+v) = %+v, %v", ref, got, err)
			}
		}
		detail, err := f.catalog.Get(svc.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(detail.Apps) != 1 || len(detail.Monitors) != 1 || len(detail.Resources) != 1 || detail.AppCount != 1 || detail.MonitorCount != 1 || detail.ResourceCount != 1 {
			t.Fatalf("detail = %+v", detail)
		}
		if detail.Health != "up" || detail.Resources[0].Key != res.Key {
			t.Fatalf("detail = %+v", detail)
		}
	})

	t.Run("linking again is a no-op and does not report a move", func(t *testing.T) {
		got, err := f.catalog.Link(svc.ID, app)
		if err != nil || got.PreviousServiceID != "" {
			t.Fatalf("got %+v, %v", got, err)
		}
	})

	t.Run("linking to another service moves it", func(t *testing.T) {
		got, err := f.catalog.Link(other.ID, mon)
		if err != nil || got.PreviousServiceID != svc.ID {
			t.Fatalf("got %+v, %v", got, err)
		}
		detail, _ := f.catalog.Get(svc.ID)
		if len(detail.Monitors) != 0 {
			t.Fatalf("monitor still on the first service: %+v", detail.Monitors)
		}
		if _, err := f.catalog.Link(svc.ID, mon); err != nil {
			t.Fatal(err)
		}
	})

	errorCases := []struct {
		name      string
		serviceID string
		ref       ServiceLinkRef
		kind      ServiceErrorKind
		contains  string
	}{
		{"missing service", "svc-missing", app, ServiceErrNotFound, "Service not found"},
		{"missing app", svc.ID, ServiceLinkRef{Kind: "app", ID: "nope"}, ServiceErrNotFound, "App not found"},
		{"missing monitor", svc.ID, ServiceLinkRef{Kind: "monitor", ID: "nope"}, ServiceErrNotFound, "Monitor not found"},
		{"deleted monitor", svc.ID, ServiceLinkRef{Kind: "monitor", ID: "mon-del"}, ServiceErrNotFound, "Monitor not found"},
		{"missing resource", svc.ID, ServiceLinkRef{Kind: "resource", NodeID: "node-1", Key: "gone"}, ServiceErrNotFound, "Resource not found"},
		{"system monitor", svc.ID, ServiceLinkRef{Kind: "monitor", ID: "mon-sys"}, ServiceErrInvalid, "System monitors"},
		{"unknown kind", svc.ID, ServiceLinkRef{Kind: "node", ID: "node-1"}, ServiceErrInvalid, "kind must be"},
		{"app without id", svc.ID, ServiceLinkRef{Kind: "app"}, ServiceErrInvalid, "id is required"},
		{"resource without key", svc.ID, ServiceLinkRef{Kind: "resource", NodeID: "node-1"}, ServiceErrInvalid, "node_id and key"},
	}
	for _, tc := range errorCases {
		t.Run("link "+tc.name, func(t *testing.T) {
			_, err := f.catalog.Link(tc.serviceID, tc.ref)
			wantServiceError(t, err, tc.kind, tc.contains)
		})
	}

	t.Run("unlink clears only the link", func(t *testing.T) {
		var before db.App
		f.database.Where("id = ?", "app-1").First(&before)
		if err := f.catalog.Unlink(svc.ID, app); err != nil {
			t.Fatal(err)
		}
		var after db.App
		if err := f.database.Where("id = ?", "app-1").First(&after).Error; err != nil {
			t.Fatalf("app is gone after unlink: %v", err)
		}
		if after.ServiceID != nil || !after.UpdatedAt.Equal(before.UpdatedAt) || after.Slug != before.Slug {
			t.Fatalf("app after unlink = %+v (before %+v)", after, before)
		}
	})

	unlinkErrors := []struct {
		name      string
		serviceID string
		ref       ServiceLinkRef
		contains  string
	}{
		{"already unlinked", svc.ID, app, "not linked to this service"},
		{"linked to a different service", other.ID, mon, "not linked to this service"},
		{"missing monitor", svc.ID, ServiceLinkRef{Kind: "monitor", ID: "nope"}, "Monitor not found"},
	}
	for _, tc := range unlinkErrors {
		t.Run("unlink "+tc.name, func(t *testing.T) {
			wantServiceError(t, f.catalog.Unlink(tc.serviceID, tc.ref), ServiceErrNotFound, tc.contains)
		})
	}
}

func TestServiceCatalogUnlinkKeepsMonitorHistory(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	svc, _ := f.catalog.Create(ServiceInput{Name: "Checkout"})
	f.monitor(t, "mon-1", "checkout health", nil)
	if err := f.database.Create(&db.MonitorReport{ID: "rep-1", MonitorID: "mon-1", Payload: "{}", CollectedAt: f.now.Format(time.RFC3339), Health: "up", CreatedAt: f.now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.database.Create(&db.Incident{ID: "inc-1", Status: "open", Severity: "high", Title: "down", NodeID: "node-1", MonitorID: "mon-1", OpenedAt: f.now, LastEventAt: f.now}).Error; err != nil {
		t.Fatal(err)
	}
	ref := ServiceLinkRef{Kind: "monitor", ID: "mon-1"}
	if _, err := f.catalog.Link(svc.ID, ref); err != nil {
		t.Fatal(err)
	}
	if err := f.catalog.Unlink(svc.ID, ref); err != nil {
		t.Fatal(err)
	}
	if _, err := f.catalog.Delete(svc.ID); err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int64{"monitors": 1, "monitor_reports": 1, "incidents": 1} {
		var n int64
		f.database.Table(table).Count(&n)
		if n != want {
			t.Fatalf("%s rows = %d, want %d", table, n, want)
		}
	}
}

func TestServiceCatalogDeleteClearsLinksAndKeepsThings(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	svc, _ := f.catalog.Create(ServiceInput{Name: "Checkout"})
	other, _ := f.catalog.Create(ServiceInput{Name: "Billing"})
	f.app(t, "app-1", "checkout")
	f.app(t, "app-2", "billing")
	f.monitor(t, "mon-1", "checkout health", nil)
	f.monitor(t, "mon-2", "billing health", nil)
	f.resource(t, "node-1", "unit:checkout", "unit", "checkout.service")
	for _, l := range []struct {
		svc string
		ref ServiceLinkRef
	}{
		{svc.ID, ServiceLinkRef{Kind: "app", ID: "app-1"}}, {svc.ID, ServiceLinkRef{Kind: "monitor", ID: "mon-1"}},
		{svc.ID, ServiceLinkRef{Kind: "resource", NodeID: "node-1", Key: "unit:checkout"}},
		{other.ID, ServiceLinkRef{Kind: "app", ID: "app-2"}}, {other.ID, ServiceLinkRef{Kind: "monitor", ID: "mon-2"}},
	} {
		if _, err := f.catalog.Link(l.svc, l.ref); err != nil {
			t.Fatal(err)
		}
	}

	got, err := f.catalog.Delete(svc.ID)
	if err != nil || got != (ServiceDeleteResult{UnlinkedApps: 1, UnlinkedMonitors: 1, UnlinkedResources: 1}) {
		t.Fatalf("Delete = %+v, %v", got, err)
	}
	_, err = f.catalog.Get(svc.ID)
	wantServiceError(t, err, ServiceErrNotFound, "Service not found")
	wantServiceError(t, func() error { _, e := f.catalog.Delete(svc.ID); return e }(), ServiceErrNotFound, "Service not found")

	var linked int64
	f.database.Model(&db.App{}).Where("service_id = ?", svc.ID).Count(&linked)
	f.database.Model(&db.Monitor{}).Where("service_id = ?", svc.ID).Count(&linked)
	f.database.Model(&db.NodeAppResource{}).Where("service_id = ?", svc.ID).Count(&linked)
	if linked != 0 {
		t.Fatalf("%d rows still point at the deleted service", linked)
	}
	var apps, monitors int64
	f.database.Model(&db.App{}).Count(&apps)
	f.database.Model(&db.Monitor{}).Count(&monitors)
	if apps != 2 || monitors != 2 {
		t.Fatalf("apps = %d, monitors = %d; deleting a service must keep them", apps, monitors)
	}
	detail, _ := f.catalog.Get(other.ID)
	if len(detail.Apps) != 1 || len(detail.Monitors) != 1 {
		t.Fatalf("the other service lost links: %+v", detail)
	}
}

func TestServiceCatalogListRollsUpHealthAndIncidents(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	healthy, _ := f.catalog.Create(ServiceInput{Name: "Alpha"})
	broken, _ := f.catalog.Create(ServiceInput{Name: "Beta"})
	empty, _ := f.catalog.Create(ServiceInput{Name: "Gamma"})
	f.monitor(t, "m-up", "up", nil)
	f.monitor(t, "m-deg", "degraded", func(m *db.Monitor) { m.Health, m.ComputedHealth = "degraded", "degraded" })
	f.monitor(t, "m-down", "down", func(m *db.Monitor) { m.Health, m.ComputedHealth, m.ActiveIncidentID = "down", "down", "inc-1" })
	f.monitor(t, "m-gone", "deleted but down", func(m *db.Monitor) { m.Lifecycle, m.Health, m.ComputedHealth = "deleted", "down", "down" })
	for svc, ids := range map[string][]string{healthy.ID: {"m-up"}, broken.ID: {"m-deg", "m-down", "m-gone"}} {
		for _, id := range ids {
			if _, err := f.catalog.Link(svc, ServiceLinkRef{Kind: "monitor", ID: id}); err != nil {
				// a deleted monitor cannot be linked through the API; link it directly to prove it is ignored
				f.database.Model(&db.Monitor{}).Where("id = ?", id).UpdateColumn("service_id", svc)
			}
		}
	}

	list, err := f.catalog.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].ID != healthy.ID || list[1].ID != broken.ID || list[2].ID != empty.ID {
		t.Fatalf("list order = %+v", list)
	}
	want := map[string]struct {
		health   string
		monitors int
		incident int
	}{healthy.ID: {"up", 1, 0}, broken.ID: {"down", 2, 1}, empty.ID: {"unknown", 0, 0}}
	for _, s := range list {
		w := want[s.ID]
		if s.Health != w.health || s.MonitorCount != w.monitors || s.OpenIncidentCount != w.incident {
			t.Fatalf("%s = %+v, want %+v", s.Name, s, w)
		}
	}
}

func TestStoreAppResourcesKeepsServiceLinkForReportedKeys(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	svc, _ := f.catalog.Create(ServiceInput{Name: "Checkout"})
	nodes := NewNodeService(f.database, utils.NewLogger())
	report := func(keys ...string) json.RawMessage {
		series := make([]map[string]any, 0, len(keys))
		for _, k := range keys {
			series = append(series, map[string]any{"kind": "container", "key": k, "name": k, "cpu_percent": 5.0})
		}
		raw, _ := json.Marshal(map[string]any{"interval_seconds": 60, "series": series})
		return raw
	}
	if err := nodes.StoreAppResources("node-1", report("container:a", "container:b")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.catalog.Link(svc.ID, ServiceLinkRef{Kind: "resource", NodeID: "node-1", Key: "container:a"}); err != nil {
		t.Fatal(err)
	}

	// The next report replaces the set but still carries the link for a.
	if err := nodes.StoreAppResources("node-1", report("container:a", "container:b")); err != nil {
		t.Fatal(err)
	}
	view, err := nodes.GetAppResources("node-1")
	if err != nil {
		t.Fatal(err)
	}
	links := map[string]string{}
	for _, s := range view.Series {
		links[s.Key] = s.ServiceID
	}
	if links["container:a"] != svc.ID || links["container:b"] != "" {
		t.Fatalf("links after a second report = %v", links)
	}

	// A key that stops being reported loses its row and its link; it does not come back linked.
	if err := nodes.StoreAppResources("node-1", report("container:b")); err != nil {
		t.Fatal(err)
	}
	if err := nodes.StoreAppResources("node-1", report("container:a", "container:b")); err != nil {
		t.Fatal(err)
	}
	view, _ = nodes.GetAppResources("node-1")
	for _, s := range view.Series {
		if s.ServiceID != "" {
			t.Fatalf("%s came back linked to %s", s.Key, s.ServiceID)
		}
	}
}
