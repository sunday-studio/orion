package logstore

import (
	"reflect"
	"testing"
	"time"
)

func TestRecentServiceNamesReadsLiteralAndNestedResourceKeys(t *testing.T) {
	store := newTestStore(t, 0)
	now := time.Now().UTC()
	withResource := func(app string, i int, at time.Time, resource map[string]any) Entry {
		e := mkEntry(app, i, at)
		e.Resource = resource
		return e
	}
	batch := []Entry{
		withResource("app-1", 0, now.Add(-time.Minute), map[string]any{"service.name": "checkout-api"}),
		withResource("app-1", 1, now.Add(-2*time.Minute), map[string]any{"service.name": "checkout-api"}),
		withResource("app-1", 2, now.Add(-3*time.Minute), map[string]any{"service": map[string]any{"name": "checkout-worker"}}),
		withResource("app-2", 3, now.Add(-time.Minute), map[string]any{"service.name": "billing"}),
		withResource("app-3", 4, now.Add(-time.Minute), map[string]any{"host.name": "box-1"}), // no service.name
		mkEntry("app-4", 5, now.Add(-time.Minute)),                                            // no resource at all
		withResource("app-5", 6, now.Add(-48*time.Hour), map[string]any{"service.name": "ancient"}),
	}
	if err := store.Enqueue(batch); err != nil {
		t.Fatal(err)
	}
	store.Flush(5 * time.Second)

	got, err := store.RecentServiceNames(now.Add(-24*time.Hour), 100)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"app-1": {"checkout-api", "checkout-worker"}, "app-2": {"billing"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RecentServiceNames = %v, want %v", got, want)
	}

	// Only the newest scanLimit entries are inspected, which bounds the cost: the two newest
	// entries with a resource are app-3 (no service.name) and app-2.
	limited, err := store.RecentServiceNames(now.Add(-24*time.Hour), 2)
	if want := map[string][]string{"app-2": {"billing"}}; err != nil || !reflect.DeepEqual(limited, want) {
		t.Fatalf("limited = %v, %v; want %v", limited, err, want)
	}
}
