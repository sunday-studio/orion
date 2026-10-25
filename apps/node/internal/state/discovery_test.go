package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoveryBaselineNewDismissAndRemoval(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	t0 := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	if st.HasScanned() {
		t.Fatal("no baseline yet")
	}
	a, b := DiscoveryItem{ID: "service:nginx", Kind: "service", Name: "nginx"}, DiscoveryItem{ID: "port:80", Kind: "port", Name: ":80"}
	fresh, err := st.RecordScan([]DiscoveryItem{a, b}, t0, true, nil)
	if err != nil || len(fresh) != 0 || !st.HasScanned() {
		t.Fatalf("baseline scan: fresh=%v err=%v", fresh, err)
	}
	items, _ := st.ListDiscovery()
	for _, it := range items {
		if it.New {
			t.Fatalf("%s: the first scan is the starting list, not 'new on this server'", it.ID)
		}
	}

	c := DiscoveryItem{ID: "docker:cache-warmer", Kind: "container", Name: "cache-warmer"}
	fresh, _ = st.RecordScan([]DiscoveryItem{a, b, c}, t0.Add(time.Hour), false, nil)
	if len(fresh) != 1 || fresh[0] != c.ID {
		t.Fatalf("fresh = %v", fresh)
	}
	if again, _ := st.RecordScan([]DiscoveryItem{a, b, c}, t0.Add(2*time.Hour), false, nil); len(again) != 0 {
		t.Fatal("a known id is not new twice")
	}
	if err := st.MarkReviewed(); err != nil {
		t.Fatal(err)
	}

	if ok, _ := st.SetDismissed("port:80", true, t0.Add(3*time.Hour)); !ok {
		t.Fatal("dismiss must find the id")
	}
	if ok, _ := st.SetDismissed("port:9999", true, t0); ok {
		t.Fatal("unknown id must report not found")
	}
	// the app stops: undismissed items vanish, dismissed ones are remembered
	st.RecordScan([]DiscoveryItem{a}, t0.Add(4*time.Hour), false, nil)
	items, _ = st.ListDiscovery()
	ids := map[string]DiscoveryItem{}
	for _, it := range items {
		ids[it.ID] = it
	}
	if _, ok := ids["docker:cache-warmer"]; ok {
		t.Fatal("a vanished, undismissed thing should be dropped")
	}
	if d, ok := ids["port:80"]; !ok || !d.Dismissed {
		t.Fatalf("a dismissal must survive the thing disappearing: %+v", d)
	}
	st.SetDismissed("port:80", false, t0)
	if items, _ = st.ListDiscovery(); len(items) != 2 {
		t.Fatalf("restore keeps it listed: %+v", items)
	}
}

func TestAFailedKindDoesNotMakeEverythingNewAgain(t *testing.T) {
	st, _ := Open(filepath.Join(t.TempDir(), "s.db"))
	defer st.Close()
	t0 := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	web := DiscoveryItem{ID: "container:web", Kind: "container", Name: "web"}
	svc := DiscoveryItem{ID: "service:nginx", Kind: "service", Name: "nginx"}
	st.RecordScan([]DiscoveryItem{web, svc}, t0, true, nil)
	// Docker is blocked this time: containers are absent, but must not be forgotten.
	keepContainers := func(it DiscoveryItem) bool { return it.Kind != "container" }
	st.RecordScan([]DiscoveryItem{svc}, t0.Add(time.Hour), false, keepContainers)
	fresh, _ := st.RecordScan([]DiscoveryItem{web, svc}, t0.Add(2*time.Hour), false, keepContainers)
	if len(fresh) != 0 {
		t.Fatalf("after a blocked scan recovers nothing is 'new on this server': %v", fresh)
	}
}
