package logstore

import (
	"fmt"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"strings"
	"sync"
	"testing"
	"time"
)

var epoch = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

func parse(t *testing.T, contentType, body string) Parsed {
	t.Helper()
	p, err := ParseRequest(contentType, []byte(body), epoch)
	if err != nil {
		t.Fatalf("ParseRequest() error = %v", err)
	}
	return p
}

func TestNormalizeAcceptsAPlainMessageAndForgivingShapes(t *testing.T) {
	p := parse(t, "application/json", `{"body":"hello"}`)
	if len(p.Entries) != 1 || p.Entries[0].Body != "hello" || p.Entries[0].Severity != "info" || !p.Entries[0].Timestamp.Equal(epoch) {
		t.Fatalf("plain message = %+v", p)
	}
	// A logger's native JSON line: msg/level/time and unknown fields become a valid entry.
	p = parse(t, "application/json", `{"msg":"user signed up","level":"WARNING","time":"2026-10-17T11:59:00Z","user_id":42,"nested":{"a":{"b":1}}}`)
	e := p.Entries[0]
	if e.Body != "user signed up" || e.Severity != "warn" || e.Timestamp.Hour() != 11 || e.Attributes["user_id"] != int64(42) || e.Attributes["nested"] == nil {
		t.Fatalf("logger line = %+v", e)
	}
	// Event-only entries are valid; structured bodies keep their JSON.
	p = parse(t, "application/json", `[{"event":"payment.failed"},{"body":{"order":8841,"ok":false}}]`)
	if len(p.Entries) != 2 || p.Entries[0].Event != "payment.failed" || p.Entries[1].BodyJSON == "" || !strings.Contains(p.Entries[1].Body, `"order":8841`) {
		t.Fatalf("event/structured = %+v", p)
	}
}

func TestNormalizeSeverityAndTimestampForms(t *testing.T) {
	for in, want := range map[string]string{`"ERROR"`: "error", `"critical"`: "fatal", `"Information"`: "info", `17`: "error", `9`: "info", `24`: "fatal", `"bogus"`: "info", `null`: "info"} {
		p := parse(t, "application/json", fmt.Sprintf(`{"body":"x","severity":%s}`, in))
		if got := p.Entries[0].Severity; got != want {
			t.Errorf("severity %s = %q, want %q", in, got, want)
		}
	}
	for name, ts := range map[string]string{"rfc3339": `"2026-10-17T10:00:00.5Z"`, "seconds": `1792231200`, "millis": `1792231200000`, "micros": `1792231200000000`, "nanos": `1792231200000000000`} {
		p := parse(t, "application/json", fmt.Sprintf(`{"body":"x","timestamp":%s}`, ts))
		if len(p.Entries) != 1 {
			t.Fatalf("%s rejected: %+v", name, p.Errors)
		}
		if got := p.Entries[0].Timestamp; got.Year() != 2026 || got.Month() != 10 || got.Day() != 17 {
			t.Errorf("%s = %v, want 2026-10-17", name, got)
		}
	}
	p := parse(t, "application/json", `{"body":"x","timestamp":"not a time"}`)
	if len(p.Errors) != 1 || p.Errors[0].Code != "invalid_timestamp" {
		t.Fatalf("bad timestamp = %+v", p.Errors)
	}
}

func TestClockSkewIntoTheFutureIsClamped(t *testing.T) {
	p := parse(t, "application/json", `{"body":"x","timestamp":"2027-01-01T00:00:00Z"}`)
	e := p.Entries[0]
	if !e.Timestamp.Equal(epoch) || e.Attributes["orion.original_timestamp"] == nil {
		t.Fatalf("future entry = %v attrs %v, want clamped to now with the original kept", e.Timestamp, e.Attributes)
	}
}

func TestOversizedAndMalformedEntriesDoNotSinkTheBatch(t *testing.T) {
	huge := strings.Repeat("x", MaxEntryBytes+10)
	body := fmt.Sprintf(`[{"body":"ok-1"},{"body":%q},"not an object",{},{"body":"ok-2"}]`, huge)
	p := parse(t, "application/json", body)
	if len(p.Entries) != 2 || len(p.Errors) != 3 {
		t.Fatalf("entries=%d errors=%+v, want 2 accepted and 3 rejected", len(p.Entries), p.Errors)
	}
	codes := map[int]string{}
	for _, e := range p.Errors {
		codes[e.Index] = e.Code
	}
	if codes[1] != "entry_too_large" || codes[2] != "invalid_json" || codes[3] != "empty_entry" {
		t.Fatalf("error codes by index = %v", codes)
	}
}

func TestNDJSONReportsBadLinesAndKeepsGoodOnes(t *testing.T) {
	p := parse(t, "application/x-ndjson; charset=utf-8", "{\"body\":\"a\"}\n\nnot json\n{\"body\":\"b\"}\n")
	if len(p.Entries) != 2 || len(p.Errors) != 1 || p.Errors[0].Code != "invalid_json" || p.Errors[0].Index != 2 {
		t.Fatalf("ndjson = %+v", p)
	}
}

func TestParseRequestRejectsUnframeableBodies(t *testing.T) {
	if _, err := ParseRequest("text/plain", []byte("hi"), epoch); err != ErrUnsupportedMedia {
		t.Fatalf("text/plain error = %v, want ErrUnsupportedMedia", err)
	}
	for _, body := range []string{"", "   ", "42", `"str"`, "{broken"} {
		if _, err := ParseRequest("application/json", []byte(body), epoch); err == nil {
			t.Errorf("body %q accepted, want ErrInvalidBody", body)
		}
	}
}

func TestAttributeLimits(t *testing.T) {
	deep := `{"a":` + strings.Repeat(`{"a":`, MaxAttributeDepth) + `1` + strings.Repeat(`}`, MaxAttributeDepth) + `}`
	p := parse(t, "application/json", `{"body":"x","attributes":`+deep+`}`)
	if len(p.Errors) != 1 || p.Errors[0].Code != "attributes_too_deep" {
		t.Fatalf("deep = %+v", p.Errors)
	}
	var keys []string
	for i := 0; i <= MaxAttributeKeys; i++ {
		keys = append(keys, fmt.Sprintf(`"k%d":1`, i))
	}
	p = parse(t, "application/json", `{"body":"x","attributes":{`+strings.Join(keys, ",")+`}}`)
	if len(p.Errors) != 1 || p.Errors[0].Code != "too_many_attributes" {
		t.Fatalf("wide = %+v", p.Errors)
	}
}

func TestRedactionHidesSecretKeysAtAnyDepthAndHonoursExtras(t *testing.T) {
	p := parse(t, "application/json", `{"body":{"password":"hunter2","ok":1},"attributes":{"Authorization":"Bearer abc","user":{"api_key":"k","name":"Ada"},"list":[{"token":"t"}],"custom_field":"keep?"}}`)
	entry := p.Entries[0]
	NewRedactor([]string{"custom_field"}, false).Apply(&entry)
	blob := entry.Body + fmt.Sprint(entry.Attributes)
	for _, secret := range []string{"hunter2", "Bearer abc", `"k"`, "keep?"} {
		if strings.Contains(blob, secret) {
			t.Errorf("secret %q survived redaction: %s", secret, blob)
		}
	}
	user := entry.Attributes["user"].(map[string]any)
	if user["name"] != "Ada" || user["api_key"] != redacted {
		t.Fatalf("user = %v, want non-secret kept and secret redacted", user)
	}
	if !strings.Contains(entry.Body, `"ok":1`) {
		t.Fatalf("structured body lost non-secret data: %s", entry.Body)
	}
	// Disabling the defaults leaves only the extras.
	loose := p.Entries[0]
	NewRedactor(nil, true).Apply(&loose)
	if loose.Attributes["Authorization"] != "Bearer abc" {
		t.Fatal("defaults were not disabled")
	}
}

func TestAppLimiterBurstRefillAndRetryAfter(t *testing.T) {
	l := NewAppLimiter(100)
	now := epoch
	l.now = func() time.Time { return now }
	if ok, _ := l.Allow("a", 200); !ok {
		t.Fatal("burst of 2x rate should pass")
	}
	ok, wait := l.Allow("a", 50)
	if ok || wait < 400*time.Millisecond || wait > 600*time.Millisecond {
		t.Fatalf("empty bucket: ok=%t wait=%v, want ~500ms", ok, wait)
	}
	if ok, _ := l.Allow("b", 200); !ok {
		t.Fatal("another app has its own bucket")
	}
	now = now.Add(time.Second)
	if ok, _ := l.Allow("a", 100); !ok {
		t.Fatal("bucket should have refilled after a second")
	}
	if ok, _ := NewAppLimiter(0).Allow("a", 1<<20); !ok {
		t.Fatal("rate 0 disables limiting")
	}
}

func newTestStore(t *testing.T, queueMax int) *Store {
	t.Helper()
	store, err := OpenWith(testkit.NewDB(t), utils.NewLogger(), queueMax)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return store
}

func mkEntry(app string, i int, ts time.Time) Entry {
	return Entry{ID: NewEntryID(ts.Add(time.Duration(i))), AppID: app, Timestamp: ts, IngestedAt: ts, Severity: "info", SeverityNumber: 9, Body: fmt.Sprintf("message %d", i), Size: 100, Attributes: map[string]any{"i": i, "http.status_code": 500 - (i%2)*300}}
}

func TestStoreWritesQueriesAndPaginates(t *testing.T) {
	store := newTestStore(t, 0)
	var batch []Entry
	for i := 0; i < 120; i++ {
		batch = append(batch, mkEntry("app-1", i, epoch.Add(time.Duration(i)*time.Second)))
	}
	batch = append(batch, mkEntry("app-2", 0, epoch))
	if err := store.Enqueue(batch); err != nil {
		t.Fatal(err)
	}
	store.Flush(5 * time.Second)

	page1, next, err := store.List(Query{AppIDs: []string{"app-1"}, Limit: 50})
	if err != nil || len(page1) != 50 || next == "" || page1[0].Body != "message 119" {
		t.Fatalf("page1 len=%d next=%q first=%q err=%v, want 50 newest first", len(page1), next, page1[0].Body, err)
	}
	seen := map[string]bool{}
	cursor := ""
	for pages := 0; pages < 10; pages++ {
		page, nxt, err := store.List(Query{AppIDs: []string{"app-1"}, Limit: 50, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range page {
			if seen[e.ID] {
				t.Fatalf("entry %s returned twice across pages", e.ID)
			}
			seen[e.ID] = true
		}
		if nxt == "" {
			break
		}
		cursor = nxt
	}
	if len(seen) != 120 {
		t.Fatalf("paged through %d entries, want 120", len(seen))
	}

	only500, _, _ := store.List(Query{AppIDs: []string{"app-1"}, Attrs: []string{"http.status_code:500"}, Limit: 200})
	if len(only500) != 60 {
		t.Fatalf("attr filter returned %d, want 60", len(only500))
	}
	text, _, _ := store.List(Query{AppIDs: []string{"app-1"}, Text: "message 77"})
	if len(text) != 1 {
		t.Fatalf("text search returned %d, want 1", len(text))
	}
	ranged, _, _ := store.List(Query{AppIDs: []string{"app-1"}, From: epoch.Add(10 * time.Second), To: epoch.Add(19 * time.Second), Limit: 200})
	if len(ranged) != 10 {
		t.Fatalf("time range returned %d, want 10", len(ranged))
	}
	got, _ := store.Get(page1[0].ID)
	if got == nil || got.Attributes["i"] == nil {
		t.Fatalf("Get() = %+v", got)
	}
}

func TestQueryFilterSafety(t *testing.T) {
	store := newTestStore(t, 0)
	store.Enqueue([]Entry{mkEntry("a", 1, epoch)})
	store.Flush(3 * time.Second)
	for _, attr := range []string{`x"); DROP TABLE log_entries; --:1`, "novalue", `a'b:1`} {
		if _, _, err := store.List(Query{Attrs: []string{attr}}); err == nil || !strings.Contains(err.Error(), "invalid log filter") {
			t.Errorf("attr %q error = %v, want ErrBadFilter", attr, err)
		}
	}
	if _, _, err := store.List(Query{Cursor: "%%%"}); err == nil {
		t.Error("bad cursor accepted")
	}
	// LIKE wildcards in free text are literals, and SQL fragments are bound, not executed.
	rows, _, err := store.List(Query{Text: `%'; DROP TABLE log_entries; --`})
	if err != nil || len(rows) != 0 {
		t.Fatalf("hostile text: rows=%d err=%v", len(rows), err)
	}
	if n, _, _ := store.CountForApp("a"); n != 1 {
		t.Fatal("table was damaged by a hostile query")
	}
}

func TestSeverityFilters(t *testing.T) {
	store := newTestStore(t, 0)
	var batch []Entry
	for i, sev := range []string{"debug", "info", "warn", "error", "fatal"} {
		e := mkEntry("a", i, epoch.Add(time.Duration(i)*time.Second))
		e.Severity, e.SeverityNumber = sev, severityByName[sev]
		batch = append(batch, e)
	}
	store.Enqueue(batch)
	store.Flush(3 * time.Second)
	atLeastWarn, _, _ := store.List(Query{Severity: ">=warn"})
	list, _, _ := store.List(Query{Severity: "error,fatal"})
	alias, _, _ := store.List(Query{Severity: "critical"})
	if len(atLeastWarn) != 3 || len(list) != 2 || len(alias) != 1 {
		t.Fatalf(">=warn=%d error,fatal=%d critical=%d, want 3/2/1", len(atLeastWarn), len(list), len(alias))
	}
}

func TestDuplicateClientIDIsIdempotent(t *testing.T) {
	store := newTestStore(t, 0)
	a, b := mkEntry("a", 1, epoch), mkEntry("a", 2, epoch)
	a.ClientID, b.ClientID = "same", "same"
	other := mkEntry("b", 3, epoch)
	other.ClientID = "same" // ids are unique per app, not globally
	store.Enqueue([]Entry{a})
	store.Enqueue([]Entry{b, other})
	store.Flush(3 * time.Second)
	if n, _, _ := store.CountForApp("a"); n != 1 {
		t.Fatalf("app a has %d entries for one client id, want 1", n)
	}
	if n, _, _ := store.CountForApp("b"); n != 1 {
		t.Fatalf("app b has %d entries, want its own copy", n)
	}
}

func TestQueueBackpressureIsBoundedAndRecovers(t *testing.T) {
	store := newTestStore(t, 100)
	// Stall the writer by holding the database write lock open, then overfill the queue.
	hold := store.DB().Begin()
	hold.Exec("INSERT INTO log_entries (id, app_id, ts, ingested_at, severity, severity_number) VALUES ('hold','z',1,1,'info',9)")
	var full bool
	for i := 0; i < 10 && !full; i++ {
		var batch []Entry
		for j := 0; j < 60; j++ {
			batch = append(batch, mkEntry("a", i*60+j, epoch))
		}
		full = store.Enqueue(batch) == ErrQueueFull
	}
	if !full {
		t.Fatal("queue never pushed back while the writer was stalled")
	}
	if q := store.Stats().Queued; q > 100 {
		t.Fatalf("queued = %d, want at most the 100-entry budget", q)
	}
	hold.Rollback()
	store.Flush(10 * time.Second)
	if err := store.Enqueue([]Entry{mkEntry("a", 9999, epoch)}); err != nil {
		t.Fatalf("queue did not recover after the stall: %v", err)
	}
}

func TestPruneByRetentionAndStorageCapInBatches(t *testing.T) {
	store := newTestStore(t, 100000)
	var batch []Entry
	for i := 0; i < 2500; i++ { // more than one delete batch
		batch = append(batch, mkEntry("a", i, epoch.AddDate(0, 0, -30)))
	}
	for i := 0; i < 100; i++ {
		batch = append(batch, mkEntry("a", 5000+i, epoch))
	}
	for i := 0; i < 50; i++ {
		batch = append(batch, mkEntry("keep", i, epoch.AddDate(0, 0, -400)))
	}
	store.Enqueue(batch)
	store.Flush(10 * time.Second)

	result, err := store.Prune([]AppRetention{{AppID: "a", RetentionDays: 14}}, epoch)
	if err != nil || result.Expired != 2500 {
		t.Fatalf("Prune() = %+v err %v, want 2500 expired across batches", result, err)
	}
	if n, _, _ := store.CountForApp("a"); n != 100 {
		t.Fatalf("app a has %d entries left, want the 100 recent ones", n)
	}
	if n, _, _ := store.CountForApp("keep"); n != 50 {
		t.Fatalf("another app lost entries: %d", n)
	}

	// A storage cap trims the oldest entries until the app fits.
	capPolicy := []AppRetention{{AppID: "a", RetentionDays: 14, MaxStorageMB: 1}}
	big := make([]Entry, 0, 30)
	for i := 0; i < 30; i++ {
		e := mkEntry("a", 8000+i, epoch.Add(time.Duration(i)*time.Second))
		e.Size = 100 << 10
		big = append(big, e)
	}
	store.Enqueue(big)
	store.Flush(5 * time.Second)
	result, err = store.Prune(capPolicy, epoch)
	if err != nil || result.OverLimit == 0 {
		t.Fatalf("Prune(cap) = %+v err %v, want entries trimmed", result, err)
	}
	if _, bytes, _ := store.CountForApp("a"); bytes > 1<<20 {
		t.Fatalf("app still uses %d bytes over the 1 MiB cap", bytes)
	}

	removed, err := store.DeleteApp("a")
	if err != nil || removed == 0 {
		t.Fatalf("DeleteApp() = %d, %v", removed, err)
	}
	if n, _, _ := store.CountForApp("a"); n != 0 {
		t.Fatal("entries remain after DeleteApp")
	}
}

func TestConcurrentWritersAndReadersDoNotLockTheDatabase(t *testing.T) {
	store := newTestStore(t, 100000)
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_ = store.Enqueue([]Entry{mkEntry("a", w*1000+i, epoch.Add(time.Duration(i)*time.Millisecond))})
				if _, _, err := store.List(Query{AppIDs: []string{"a"}, Limit: 10}); err != nil {
					t.Errorf("list during writes: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	store.Flush(10 * time.Second)
	if st := store.Stats(); st.Dropped != 0 || st.Written != 400 {
		t.Fatalf("stats = %+v, want 400 written and none dropped", st)
	}
}
