package logship

import "testing"

func TestRecentMarkedFlagsEntriesBeforeTheCursorAsShipped(t *testing.T) {
	sp, err := OpenSpool(t.TempDir(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer sp.Close()
	if err := sp.Append([]Entry{{ID: "a", Body: "one"}, {ID: "b", Body: "two"}, {ID: "c", Body: "three"}}); err != nil {
		t.Fatal(err)
	}
	batch, cur, err := sp.ReadBatch(Cursor{}, 2, 1<<20)
	if err != nil || len(batch) != 2 {
		t.Fatalf("batch %v err %v", batch, err)
	}
	got, _ := sp.RecentMarked(10)
	for _, e := range got {
		if e.Shipped == nil || *e.Shipped {
			t.Fatalf("nothing is shipped before the cursor is saved: %+v", e)
		}
	}
	if err := sp.SaveCursor(cur); err != nil {
		t.Fatal(err)
	}
	got, _ = sp.RecentMarked(10)
	want := map[string]bool{"a": true, "b": true, "c": false}
	for _, e := range got {
		if e.Shipped == nil || *e.Shipped != want[e.ID] {
			t.Errorf("%s shipped = %v, want %v", e.ID, e.Shipped, want[e.ID])
		}
	}
}
