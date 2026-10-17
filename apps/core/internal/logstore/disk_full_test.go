package logstore

import (
	"fmt"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"
)

// Disk-full chaos: PRAGMA max_page_count makes SQLite fail writes with the same
// SQLITE_FULL ("database or disk is full") a real full volume produces, without root or a
// loop device. The store must refuse loudly, keep existing data intact, and recover.
func TestDiskFullDegradesLoudlyKeepsDataAndRecovers(t *testing.T) {
	old := storageFullBackoff
	storageFullBackoff = 200 * time.Millisecond
	t.Cleanup(func() { storageFullBackoff = old })

	database := testkit.NewDB(t)
	sqlDB, _ := database.DB()
	sqlDB.SetMaxOpenConns(1) // max_page_count is per connection; one connection makes it deterministic
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)
	store, err := OpenWith(database, utils.NewLogger(), 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)

	big := strings.Repeat("x", 1500)
	mk := func(from, n int) []Entry {
		var out []Entry
		for i := from; i < from+n; i++ {
			e := mkEntry("app", i, epoch.Add(time.Duration(i)*time.Second))
			e.Body = big
			out = append(out, e)
		}
		return out
	}
	if err := store.Enqueue(mk(0, 500)); err != nil {
		t.Fatal(err)
	}
	store.Flush(10 * time.Second)
	before, _, _ := store.CountForApp("app")
	if before != 500 {
		t.Fatalf("seeded %d rows, want 500", before)
	}

	var pages int
	database.Raw("PRAGMA page_count").Scan(&pages)
	if err := database.Exec(fmt.Sprintf("PRAGMA max_page_count = %d", pages)).Error; err != nil {
		t.Fatal(err)
	}

	if err := store.Enqueue(mk(1000, 2000)); err != nil {
		t.Fatalf("first oversized batch should be accepted into the queue: %v", err)
	}
	store.Flush(10 * time.Second)
	st := store.Stats()
	if st.Dropped == 0 || !st.StorageFull {
		t.Fatalf("stats after full disk = %+v, want dropped>0 and storage_full", st)
	}
	if err := store.Enqueue(mk(5000, 1)); err != ErrStorageFull {
		t.Fatalf("Enqueue while full = %v, want ErrStorageFull", err)
	}

	after, _, _ := store.CountForApp("app")
	if after != before {
		t.Fatalf("rows %d -> %d: a failed batch must not partially commit", before, after)
	}
	var check string
	database.Raw("PRAGMA integrity_check").Scan(&check)
	if check != "ok" {
		t.Fatalf("integrity_check = %q", check)
	}

	// Space returns (raise the cap): after the backoff the store probes and recovers.
	database.Exec("PRAGMA max_page_count = 1073741823")
	time.Sleep(storageFullBackoff + 50*time.Millisecond)
	if err := store.Enqueue(mk(6000, 10)); err != nil {
		t.Fatalf("Enqueue after recovery window = %v", err)
	}
	store.Flush(10 * time.Second)
	final, _, _ := store.CountForApp("app")
	if final != before+10 || store.Stats().StorageFull {
		t.Fatalf("rows=%d (want %d) stats=%+v: should have recovered", final, before+10, store.Stats())
	}
}
