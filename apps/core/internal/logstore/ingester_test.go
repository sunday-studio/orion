package logstore

import (
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"strings"
	"testing"
	"time"
)

func newIngester(t *testing.T, rate int) (*Ingester, *Store, db.App, string) {
	t.Helper()
	main := testkit.NewDB(t)
	store := newTestStore(t, 0)
	app := db.App{ID: "app-1", Name: "checkout", Slug: "checkout", RetentionDays: 14, DebugSampleRate: 1, DisplayJSON: "{}", RedactionJSON: `{"extra_keys":["internal_note"]}`, CreatedAt: epoch, UpdatedAt: epoch}
	if err := main.Create(&app).Error; err != nil {
		t.Fatal(err)
	}
	key, hash, prefix, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := main.Create(&db.AppIngestKey{ID: "key-1", AppID: app.ID, Name: "prod", Prefix: prefix, KeyHash: hash, CreatedAt: epoch}).Error; err != nil {
		t.Fatal(err)
	}
	return NewIngester(main, store, NewAppLimiter(rate)), store, app, key
}

func TestIngesterAuthenticatesRevokesAndIsolatesApps(t *testing.T) {
	ing, store, app, key := newIngester(t, 0)
	ctx, err := ing.Authenticate("Bearer " + key)
	if err != nil || ctx.App.ID != app.ID {
		t.Fatalf("Authenticate() = %+v, %v", ctx, err)
	}
	for _, bad := range []string{"", "Bearer ", "Bearer nope", "Bearer orion_ik_deadbeef", key + "x"} {
		if _, err := ing.Authenticate(bad); err != ErrInvalidKey {
			t.Errorf("Authenticate(%q) = %v, want ErrInvalidKey", bad, err)
		}
	}
	res, err := ing.Ingest(ctx, "application/json", []byte(`{"body":"hello","attributes":{"password":"p","internal_note":"n","ok":1}}`))
	if err != nil || res.Accepted != 1 {
		t.Fatalf("Ingest() = %+v, %v", res, err)
	}
	store.Flush(3 * time.Second)
	rows, _, _ := store.List(Query{})
	if len(rows) != 1 || rows[0].AppID != app.ID {
		t.Fatalf("stored = %+v, want the entry stamped with the key's app", rows)
	}
	if rows[0].Attributes["password"] != redacted || rows[0].Attributes["internal_note"] != redacted || rows[0].Attributes["ok"] == nil {
		t.Fatalf("attributes = %v, want default and per-app redaction", rows[0].Attributes)
	}

	// Revocation takes effect once the cache is invalidated.
	ing.main.Model(&db.AppIngestKey{}).Where("id = ?", "key-1").Update("revoked_at", time.Now().UTC().Add(-time.Second))
	ing.Invalidate()
	if _, err := ing.Authenticate(key); err != ErrInvalidKey {
		t.Fatalf("revoked key error = %v, want ErrInvalidKey", err)
	}
}

func TestIngesterRateLimitsPerAppWithRetryAfter(t *testing.T) {
	ing, _, _, key := newIngester(t, 10)
	ctx, _ := ing.Authenticate(key)
	var body []string
	for i := 0; i < 20; i++ {
		body = append(body, `{"body":"x"}`)
	}
	payload := []byte("[" + strings.Join(body, ",") + "]")
	if _, err := ing.Ingest(ctx, "application/json", payload); err != nil {
		t.Fatalf("burst error = %v", err)
	}
	_, err := ing.Ingest(ctx, "application/json", payload)
	var limited *RateLimitedError
	if !errorsAs(err, &limited) || limited.RetryAfter <= 0 {
		t.Fatalf("second burst error = %v, want RateLimitedError with Retry-After", err)
	}
}

func TestIngesterSamplesDebugButNeverErrors(t *testing.T) {
	ing, store, app, key := newIngester(t, 0)
	ing.main.Model(&db.App{}).Where("id = ?", app.ID).Update("debug_sample_rate", 0.0001)
	ing.Invalidate()
	ctx, _ := ing.Authenticate(key)
	var body []string
	for i := 0; i < 200; i++ {
		body = append(body, `{"body":"d","severity":"debug"}`, `{"body":"e","severity":"error"}`)
	}
	res, err := ing.Ingest(ctx, "application/json", []byte("["+strings.Join(body, ",")+"]"))
	if err != nil || res.Accepted != 400 {
		t.Fatalf("Ingest() = %+v, %v, want all 400 reported accepted", res, err)
	}
	store.Flush(3 * time.Second)
	errs, _, _ := store.List(Query{Severity: "error", Limit: 200})
	debugs, _, _ := store.List(Query{Severity: "debug", Limit: 200})
	if len(errs) != 200 || len(debugs) > 5 {
		t.Fatalf("stored errors=%d debug=%d, want every error kept and debug sampled away", len(errs), len(debugs))
	}
}

func errorsAs(err error, target **RateLimitedError) bool {
	for err != nil {
		if e, ok := err.(*RateLimitedError); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
