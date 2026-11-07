package service

import (
	"fmt"
	"orion/core/internal/db"
	"sort"
	"testing"
)

func TestServiceCatalogUpdateTouchesOnlyPatchedFields(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	created, err := f.catalog.Create(ServiceInput{Name: "Checkout", NodeID: "node-1", Environment: "prod", RunbookURL: "https://wiki.example.com/x"})
	if err != nil {
		t.Fatal(err)
	}
	name, env := "Checkout API", "staging"
	// Two patches of different fields, applied one after the other, must both stick.
	if _, err := f.catalog.Update(created.ID, ServicePatch{Name: &name}); err != nil {
		t.Fatal(err)
	}
	got, err := f.catalog.Update(created.ID, ServicePatch{Environment: &env})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != name || got.Environment != env || got.NodeID != "node-1" || got.RunbookURL != "https://wiki.example.com/x" || got.Slug != created.Slug {
		t.Fatalf("a patch overwrote a field it did not name: %+v", got)
	}
}

func TestServiceCatalogUpdateNeverRecreatesADeletedService(t *testing.T) {
	f := newCatalogFixture(t)
	created, err := f.catalog.Create(ServiceInput{Name: "Checkout"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.catalog.Delete(created.ID); err != nil {
		t.Fatal(err)
	}
	name := "Back from the dead"
	_, err = f.catalog.Update(created.ID, ServicePatch{Name: &name})
	wantServiceError(t, err, ServiceErrNotFound, "")
	var n int64
	f.database.Model(&db.Service{}).Count(&n)
	if n != 0 {
		t.Fatalf("Update recreated a deleted service (%d rows)", n)
	}
}

func TestServiceSuggestionsCapAndOrderManyMatches(t *testing.T) {
	f := newCatalogFixture(t)
	f.node(t, "node-1")
	svc, err := f.catalog.Create(ServiceInput{Name: "Checkout API"})
	if err != nil {
		t.Fatal(err)
	}
	const total = 400
	for i := 0; i < total; i++ {
		f.monitor(t, fmt.Sprintf("mon-%03d", i), fmt.Sprintf("checkout-api check %03d", i), nil)
	}
	got, err := f.catalog.Suggest(svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want one group, got %d", len(got))
	}
	members := got[0].Members
	if len(members) != suggestionMaxMembers {
		t.Fatalf("members = %d, want the cap %d", len(members), suggestionMaxMembers)
	}
	names := make([]string, len(members))
	for i, m := range members {
		names[i] = m.Name
	}
	if !sort.StringsAreSorted(names) || names[0] != "checkout-api check 000" {
		t.Fatalf("members should be the first %d by name, got %v", suggestionMaxMembers, names)
	}
}
