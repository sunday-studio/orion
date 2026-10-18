package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const sendSnippetBytes = 300

// SendRecord captures what the receiving end answered, for attempt history and test results.
// A nil *SendRecord is valid and ignored.
type SendRecord struct {
	StatusCode int
	Snippet    string
	// Delivered and Skipped count email recipients.
	Delivered int
	Skipped   int
}

func (r *SendRecord) fromResponse(resp *http.Response) {
	if r == nil {
		return
	}
	r.StatusCode = resp.StatusCode
	body, _ := io.ReadAll(io.LimitReader(resp.Body, sendSnippetBytes))
	r.Snippet = strings.TrimSpace(string(body))
}

// ScrubSnippet removes custom header values from a stored response snippet, so a receiver that
// echoes the request cannot expose them through the attempt history.
func (r *SendRecord) ScrubSnippet(headers map[string]string) {
	if r == nil {
		return
	}
	for _, value := range headers {
		if len(value) >= 4 {
			r.Snippet = strings.ReplaceAll(r.Snippet, value, "[redacted]")
		}
	}
}

// forbiddenChannelHeaders cannot be set on a webhook: they would break framing or spoof Orion's own headers.
var forbiddenChannelHeaders = map[string]bool{"host": true, "content-length": true, "content-type": true, "transfer-encoding": true, "connection": true, "user-agent": true}

// DecodeChannelHeaders reads stored custom headers.
func DecodeChannelHeaders(raw string) map[string]string {
	out := map[string]string{}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// ValidateChannelHeaders checks custom header names and values.
func ValidateChannelHeaders(headers map[string]string) error {
	if len(headers) > 10 {
		return fmt.Errorf("at most 10 custom headers")
	}
	for name, value := range headers {
		lower := strings.ToLower(name)
		if !headerNamePattern.MatchString(name) || forbiddenChannelHeaders[lower] || strings.HasPrefix(lower, "x-orion-") {
			return fmt.Errorf("header %q is not allowed", name)
		}
		if len(value) > 500 || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("header %q has an invalid value", name)
		}
	}
	return nil
}

var headerNamePattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,60}$`)
