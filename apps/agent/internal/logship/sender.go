package logship

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Sender delivers one batch of entries for one source.
type Sender interface {
	Send(ctx context.Context, source string, entries []Entry) error
}

// ErrRejected means Core refused the batch permanently (bad key, bad payload);
// retrying cannot help, so the shipper drops it and records why.
var ErrRejected = errors.New("rejected by Core")

// RetryAfterError asks the caller to wait before retrying (429 backpressure).
type RetryAfterError struct{ After time.Duration }

func (e *RetryAfterError) Error() string { return fmt.Sprintf("Core asked to retry after %s", e.After) }

// HTTPSender posts NDJSON to Core's native ingest endpoint with a per-source
// write-only ingest key (docs/plans/product-logs-plan.md section 3).
type HTTPSender struct {
	URL    string
	Key    func(source string) string
	Client *http.Client
}

func (h *HTTPSender) Send(ctx context.Context, source string, entries []Entry) error {
	key := ""
	if h.Key != nil {
		key = h.Key(source)
	}
	if key == "" {
		return fmt.Errorf("%w: no ingest key configured for source %q", ErrRejected, source)
	}
	var body bytes.Buffer
	for _, e := range entries {
		out := e
		out.Source, out.Shipped = "", nil // spool-only fields
		b, err := json.Marshal(out)
		if err != nil {
			continue
		}
		body.Write(b)
		body.WriteByte('\n')
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	req.Header.Set("Authorization", "Bearer "+key)
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	switch {
	case resp.StatusCode/100 == 2:
		return nil
	case resp.StatusCode == http.StatusTooManyRequests:
		wait := 5 * time.Second
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
			wait = time.Duration(s) * time.Second
		}
		return &RetryAfterError{After: wait}
	case resp.StatusCode >= 500:
		return fmt.Errorf("core returned HTTP %d", resp.StatusCode)
	default: // 4xx other than 429
		return fmt.Errorf("%w: HTTP %d %s", ErrRejected, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
}
