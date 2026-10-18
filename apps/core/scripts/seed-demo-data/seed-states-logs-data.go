package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"orion/core/internal/logstore"
)

var seedEntrySeq int

// makeEntry normalises an ingest-style object the way the ingest endpoint does, then stamps it for an app.
func makeEntry(appID string, at time.Time, object map[string]any) (logstore.Entry, bool) {
	raw, _ := json.Marshal(object)
	object["timestamp"] = at.UTC().Format(time.RFC3339Nano)
	entry, err := logstore.Normalize(object, len(raw), at)
	if err != nil {
		return logstore.Entry{}, false
	}
	seedEntrySeq++
	entry.ID = fmt.Sprintf("seed-log-%016x-%06d", at.UnixNano(), seedEntrySeq)
	entry.AppID = appID
	entry.Timestamp, entry.IngestedAt = at.UTC(), at.UTC()
	if entry.Size < 700 {
		entry.Size = 700 + int(at.UnixNano()%300)
	}
	return entry, true
}

// offsets spreads n timestamps: most in the last day, some older, and a dense burst in the last minute (the "2 s ago" feel).
func offsets(r *rand.Rand, now time.Time, n, lastMinute int) []time.Time {
	out := make([]time.Time, 0, n+lastMinute)
	for i := 0; i < n; i++ {
		var d time.Duration
		switch x := r.Float64(); {
		case x < 0.6:
			d = time.Duration(r.Int63n(int64(24 * time.Hour)))
		case x < 0.85:
			d = 24*time.Hour + time.Duration(r.Int63n(int64(48*time.Hour)))
		default:
			d = 72*time.Hour + time.Duration(r.Int63n(int64(96*time.Hour)))
		}
		out = append(out, now.Add(-d-time.Minute))
	}
	for i := 0; i < lastMinute; i++ {
		out = append(out, now.Add(-time.Duration(r.Int63n(int64(58*time.Second)))-time.Second))
	}
	return out
}

func billingEntries(now time.Time) []logstore.Entry {
	r := rand.New(rand.NewSource(11))
	appID := appIDFor("billing-api")
	resource := map[string]any{"service.name": "billing-api", "service.version": "2.14.0", "deployment.environment": "production"}
	var out []logstore.Entry
	add := func(at time.Time, obj map[string]any) {
		obj["resource"] = resource
		if e, ok := makeEntry(appID, at, obj); ok {
			out = append(out, e)
		}
	}
	for i, at := range offsets(r, now, 1100, 40) {
		req := fmt.Sprintf("req_%06x", r.Intn(1<<24))
		trace := fmt.Sprintf("%032x", r.Int63())
		user := fmt.Sprintf("u_%d", 10+r.Intn(90))
		amount := float64(1000+r.Intn(20000)) / 100
		base := func(extra map[string]any) map[string]any {
			a := map[string]any{"user": map[string]any{"id": user}, "amount": amount, "plan": []string{"free", "pro", "team"}[r.Intn(3)]}
			for k, v := range extra {
				a[k] = v
			}
			return a
		}
		fail := r.Float64() < 0.07
		add(at, map[string]any{"severity": "info", "event": "payment.started", "body": "charging order " + fmt.Sprint(8000+i), "request_id": req, "trace_id": trace,
			"attributes": base(map[string]any{"http": map[string]any{"route": "/orders/charge", "status": 202}, "card_number": "[redacted]", "authorization": "[redacted]"})})
		if fail {
			status := []int{502, 504, 503}[r.Intn(3)]
			add(at.Add(28*time.Millisecond), map[string]any{"severity": "error", "event": "payment.failed", "body": fmt.Sprintf("upstream returned %d for order %d", status, 8000+i), "request_id": req, "trace_id": trace,
				"attributes": base(map[string]any{"http": map[string]any{"route": "/orders/charge", "status": status}, "duration_ms": 1800 + r.Intn(8000), "internal": map[string]any{"retry": true}})})
			add(at.Add(300*time.Millisecond), map[string]any{"severity": "warn", "event": "card.retry", "body": fmt.Sprintf("retry %d of 3", 1+r.Intn(3)), "request_id": req, "trace_id": trace, "attributes": base(nil)})
		} else {
			add(at.Add(41*time.Millisecond), map[string]any{"severity": "info", "event": "payment.succeeded", "body": "payment accepted", "request_id": req, "trace_id": trace,
				"attributes": base(map[string]any{"http": map[string]any{"route": "/orders/charge", "status": 200}, "duration_ms": 90 + r.Intn(400)})})
			add(at.Add(70*time.Millisecond), map[string]any{"severity": "info", "event": "invoice.created", "body": fmt.Sprintf("invoice inv_%d created", 4400+i), "request_id": req, "trace_id": trace, "attributes": base(nil)})
			add(at.Add(95*time.Millisecond), map[string]any{"severity": "info", "event": "email.queued", "body": "receipt email queued", "request_id": req, "trace_id": trace, "attributes": base(nil)})
		}
		if r.Float64() < 0.04 {
			add(at.Add(time.Second), map[string]any{"severity": "debug", "event": "cache.miss", "body": "plan cache miss", "attributes": base(nil)})
		}
	}
	// A failure that only started recently, so Patterns can flag it as new.
	for i := 0; i < 6; i++ {
		add(now.Add(-time.Duration(2+i*4)*time.Minute), map[string]any{"severity": "error", "event": "card.declined", "body": fmt.Sprintf("issuer unavailable (code %d)", 91), "attributes": map[string]any{"user": map[string]any{"id": fmt.Sprintf("u_%d", 20+i)}, "issuer": "bank-" + fmt.Sprint(i%3)}})
	}
	return out
}

func webStoreEntries(now time.Time) []logstore.Entry {
	r := rand.New(rand.NewSource(23))
	appID := appIDFor("web-store")
	var out []logstore.Entry
	for _, at := range offsets(r, now, 1500, 30) {
		var obj map[string]any
		route := []string{"/", "/cart", "/checkout", "/products/42", "/search"}[r.Intn(5)]
		switch x := r.Float64(); {
		case x < 0.45:
			obj = map[string]any{"severity": "info", "event": "page.view", "body": "GET " + route, "attributes": map[string]any{"route": route, "duration_ms": 20 + r.Intn(180)}}
		case x < 0.75:
			obj = map[string]any{"severity": "debug", "event": "cache.hit", "body": "cache hit", "attributes": map[string]any{"key": fmt.Sprintf("cart:u_%d", r.Intn(90)), "route": route}}
		case x < 0.88:
			obj = map[string]any{"severity": "warn", "event": "checkout.slow", "body": "checkout took longer than 1s", "attributes": map[string]any{"route": "/checkout", "duration_ms": 1200 + r.Intn(2600)}}
		case x < 0.97:
			obj = map[string]any{"severity": "info", "event": "order.created", "body": "order created", "attributes": map[string]any{"order_id": 9000 + r.Intn(5000), "total": float64(r.Intn(40000)) / 100}}
		default:
			obj = map[string]any{"severity": "error", "event": "checkout.error", "body": "payment form failed to load", "attributes": map[string]any{"route": "/checkout", "browser": []string{"Safari 17", "Chrome 126", "Firefox 128"}[r.Intn(3)]}}
		}
		if e, ok := makeEntry(appID, at, obj); ok {
			out = append(out, e)
		}
	}
	return out
}

// cronEntries are plain text lines with no structure; the severity is read from the first words.
func cronEntries(now time.Time) []logstore.Entry {
	appID := appIDFor("legacy-cron")
	var out []logstore.Entry
	for day := 0; day < 9; day++ {
		start := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 1, 120_000_000, time.UTC).AddDate(0, 0, -day)
		if start.After(now.Add(-3 * time.Hour)) {
			start = start.AddDate(0, 0, -1) // the newest run finished three hours ago
		}
		lines := []string{"Starting backup job nightly-sync", "Dumping database orders (2.1 GB)", "WARN slow disk, 14 MB/s", fmt.Sprintf("Uploading to s3://backups/%s", start.Format("2006-01-02")), "Backup finished in 41s, 2.1 GB"}
		if day == 1 {
			lines = []string{"Starting backup job nightly-sync", "Dumping database orders (2.1 GB)", "WARN slow disk, 14 MB/s", "Uploading to s3://backups/" + start.Format("2006-01-02"), "ERROR upload failed: connection reset by peer", "  at s3.upload (backup.sh:41)", "Retrying in 30s (attempt 2)", "Backup finished in 118s, 2.1 GB"}
		}
		for i, line := range lines {
			if e, ok := makeEntry(appID, start.Add(time.Duration(i)*7*time.Second), map[string]any{"body": line}); ok {
				out = append(out, e)
			}
		}
	}
	return out
}

func gatewayEntries(now time.Time) []logstore.Entry {
	r := rand.New(rand.NewSource(37))
	appID := appIDFor("edge-gateway")
	var out []logstore.Entry
	for _, at := range offsets(r, now, 420, 6) {
		upstream := []string{"billing-api", "web-store", "search", "auth"}[r.Intn(4)]
		var obj map[string]any
		switch x := r.Float64(); {
		case x < 0.7:
			obj = map[string]any{"severity": "info", "event": "request.proxied", "body": map[string]any{"upstream": upstream, "status": 200, "ms": 12 + r.Intn(90)}}
		case x < 0.85:
			obj = map[string]any{"severity": "warn", "event": "tls.handshake", "body": map[string]any{"upstream": upstream, "note": "slow handshake", "ms": 600 + r.Intn(900)}}
		case x < 0.95:
			obj = map[string]any{"severity": "error", "event": "upstream.timeout", "body": map[string]any{"upstream": upstream, "status": 504, "timeout_ms": 10000}}
		case x < 0.99:
			obj = map[string]any{"severity": "info", "event": "config.reload", "body": map[string]any{"routes": 41, "changed": 2}}
		default:
			obj = map[string]any{"severity": "fatal", "event": "panic.recovered", "body": map[string]any{"error": "nil pointer dereference", "handler": "router.dispatch"}}
		}
		if e, ok := makeEntry(appID, at, obj); ok {
			out = append(out, e)
		}
	}
	return out
}

// internalEntries are Core's own log lines for the Internal logs tab.
func internalEntries(now time.Time) []logstore.Entry {
	r := rand.New(rand.NewSource(5))
	type line struct {
		severity, component, msg string
		extra                    map[string]any
	}
	lines := []line{
		{"info", "agent gateway", "Agent report accepted", map[string]any{"server": "edge-01", "monitors": 5}},
		{"warn", "agent gateway", "Slow agent report", map[string]any{"server": "edge-03", "latency_ms": 1204}},
		{"error", "worker", "Alert delivery retry failed", map[string]any{"channel": "oncall webhook", "attempt": 3, "error": "webhook returned status 502"}},
		{"info", "core api", "Incident opened", map[string]any{"incident": "INC-0231"}},
		{"warn", "scheduler", "Check delayed", map[string]any{"queue_depth": 41}},
		{"info", "worker", "Rollup finished", map[string]any{"days": 1204, "seconds": 3.1}},
		{"error", "core api", "HTTP request", map[string]any{"method": "GET", "route": "/v1/overview", "status": 500, "duration_ms": 31}},
		{"warn", "core api", "HTTP request", map[string]any{"method": "GET", "route": "/v1/status-logs", "status": 200, "duration_ms": 1480}},
		{"info", "scheduler", "Data lifecycle scheduler ran", map[string]any{"archived_reports": 18230}},
	}
	var out []logstore.Entry
	for i := 0; i < 120; i++ {
		l := lines[r.Intn(len(lines))]
		at := now.Add(-time.Duration(r.Int63n(int64(6 * time.Hour))))
		attrs := map[string]any{"component": l.component, "process": map[bool]string{true: "worker", false: "api"}[l.component == "worker"]}
		for k, v := range l.extra {
			attrs[k] = v
		}
		if e, ok := makeEntry(logstore.InternalAppID, at, map[string]any{"severity": l.severity, "body": l.msg, "attributes": attrs}); ok {
			e.ID = fmt.Sprintf("seed-internal-%016x-%03d", at.UnixNano(), i)
			out = append(out, e)
		}
	}
	return out
}
