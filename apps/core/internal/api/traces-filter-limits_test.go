package api

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTraceSearchWithoutToAllowsForClocksSlightlyAhead(t *testing.T) {
	server, _ := tracesServer(t, nil, 0)
	_, key := createAppWithKey(t, server, "checkout-api")
	now := time.Now().UTC()
	sendTraces(t, server, server.traces, key,
		otlpSpan(1, 1, 0, "recent", now.Add(-10*time.Minute), time.Second, false),
		otlpSpan(2, 2, 0, "ahead 2m", now.Add(2*time.Minute), time.Second, false), // inside the five minute allowance
		otlpSpan(3, 3, 0, "ahead 30m", now.Add(30*time.Minute), time.Second, false))

	got := traceRootNames(getTraces(t, server, "", http.StatusOK))
	if want := []string{"ahead 2m", "recent"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("no to: %v, want %v (default to is now plus 5 minutes)", got, want)
	}
	// An explicit to is taken as given.
	got = traceRootNames(getTraces(t, server, "to=now", http.StatusOK))
	if want := []string{"recent"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("to=now: %v, want %v", got, want)
	}
	explicit := url.QueryEscape(now.Add(time.Hour).Format(time.RFC3339))
	if got := traceRootNames(getTraces(t, server, "to="+explicit, http.StatusOK)); len(got) != 3 {
		t.Fatalf("to an hour ahead: %v, want all three", got)
	}
	// The facets count over the same window.
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/traces/facets", nil, "")
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"total":2`) {
		t.Fatalf("facets = %d %s, want a total of 2", resp.Code, resp.Body.String())
	}
}

func TestTraceFiltersCapTheAppAndAttributeLists(t *testing.T) {
	server, _ := tracesServer(t, nil, 0)
	ids := func(n int) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = fmt.Sprintf("app-%d", i)
		}
		return strings.Join(parts, ",")
	}
	attrs := func(n int) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = fmt.Sprintf("attr=k%d:v", i)
		}
		return strings.Join(parts, "&")
	}
	for _, path := range []string{"/v1/traces", "/v1/traces/facets"} {
		for name, tc := range map[string]struct {
			query string
			want  int
		}{
			"50 apps":         {"app_id=" + ids(50), http.StatusOK},
			"51 apps":         {"app_id=" + ids(51), http.StatusBadRequest},
			"50 attributes":   {attrs(50), http.StatusOK},
			"51 attributes":   {attrs(51), http.StatusBadRequest},
			"thousands":       {"app_id=" + ids(5000), http.StatusBadRequest},
			"both at the cap": {"app_id=" + ids(50) + "&" + attrs(50), http.StatusOK},
		} {
			resp := performJSONRequest(t, server, http.MethodGet, path+"?"+tc.query, nil, "")
			if resp.Code != tc.want {
				t.Errorf("%s %s = %d, want %d: %.200s", path, name, resp.Code, tc.want, resp.Body.String())
			}
			if tc.want == http.StatusBadRequest && !strings.Contains(resp.Body.String(), "at most 50") {
				t.Errorf("%s %s: the message must say the limit: %s", path, name, resp.Body.String())
			}
		}
	}
}
