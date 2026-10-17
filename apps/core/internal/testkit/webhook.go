package testkit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Webhook is a scriptable alert receiver bound to loopback (tests must enable
// ORION_ALERT_ALLOW_PRIVATE_WEBHOOKS equivalent via config).
type Webhook struct {
	Server *httptest.Server

	mu       sync.Mutex
	status   func(call int) int
	delay    time.Duration
	redirect string
	bodies   [][]byte
	calls    atomic.Int32
}

// NewWebhook starts a receiver that answers 204 until scripted otherwise.
func NewWebhook(t testing.TB) *Webhook {
	t.Helper()
	w := &Webhook{status: func(int) int { return http.StatusNoContent }}
	w.Server = httptest.NewServer(http.HandlerFunc(w.handle))
	t.Cleanup(w.Server.Close)
	return w
}

func (w *Webhook) URL() string { return w.Server.URL }

// Calls is the number of requests received.
func (w *Webhook) Calls() int { return int(w.calls.Load()) }

// Bodies returns copies of the request bodies received so far.
func (w *Webhook) Bodies() [][]byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([][]byte(nil), w.bodies...)
}

// FailFirst makes the first n calls return 500, then 204.
func (w *Webhook) FailFirst(n int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = func(call int) int {
		if call <= n {
			return http.StatusInternalServerError
		}
		return http.StatusNoContent
	}
}

// AlwaysFail makes every call return 500.
func (w *Webhook) AlwaysFail() { w.FailFirst(1 << 30) }

// Delay makes the receiver hold each response for d.
func (w *Webhook) Delay(d time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.delay = d
}

// RedirectTo makes every call answer 302 to url.
func (w *Webhook) RedirectTo(url string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.redirect = url
}

func (w *Webhook) handle(rw http.ResponseWriter, r *http.Request) {
	call := int(w.calls.Add(1))
	body, _ := io.ReadAll(r.Body)
	w.mu.Lock()
	w.bodies = append(w.bodies, body)
	status, delay, redirect := w.status, w.delay, w.redirect
	w.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	if redirect != "" {
		http.Redirect(rw, r, redirect, http.StatusFound)
		return
	}
	rw.WriteHeader(status(call))
}
