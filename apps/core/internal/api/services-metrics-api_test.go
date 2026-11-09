package api

import (
	"net/http"
	"orion/core/internal/metricstore"
	"orion/core/internal/utils"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func serverWithMetrics(t *testing.T) (*Server, *metricstore.Store) {
	t.Helper()
	server := setupTestServer(t)
	store, err := metricstore.Open(t.TempDir(), utils.NewLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	server.EnableMetrics(store)
	return server, store
}

type serviceMetricsBody struct {
	Data struct {
		ServiceID  string `json:"service_id"`
		Resolution string `json:"resolution"`
		Series     []struct {
			Source string `json:"source"`
			NodeID string `json:"node_id"`
			Key    string `json:"key"`
			Name   string `json:"name"`
			Points []struct {
				CPUPercent  *float64 `json:"cpu_percent"`
				MemoryBytes *float64 `json:"memory_bytes"`
				IOReadBps   *float64 `json:"io_read_bytes_per_sec"`
			} `json:"points"`
		} `json:"series"`
	} `json:"data"`
}

func TestServiceMetricsReturnsHistoryOfLinkedResources(t *testing.T) {
	server, store := serverWithMetrics(t)
	node := registerTestNode(t, server)
	nodeID := node.Data.NodeID
	reportWithAppResources(t, server, nodeID, node.Data.Token, map[string]any{
		"interval_seconds": 60,
		"series": []map[string]any{
			{"kind": "container", "key": "container:web", "name": "web", "cpu_percent": 12.5, "memory_bytes": 1048576},
			{"kind": "container", "key": "container:db", "name": "db", "cpu_percent": 3.0},
		},
	})
	store.Flush()

	id := createServiceVia(t, server, gin.H{"name": "Checkout"})
	// Nothing is linked yet: the service has no series, though samples were stored.
	var empty serviceMetricsBody
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, metricsHistoryPath(id), nil, ""), &empty)
	if len(empty.Data.Series) != 0 {
		t.Fatalf("unlinked service has series: %+v", empty.Data.Series)
	}

	link := gin.H{"kind": "resource", "node_id": nodeID, "key": "container:web"}
	if resp := performJSONRequest(t, server, http.MethodPost, "/v1/services/"+id+"/links", link, ""); resp.Code != http.StatusOK {
		t.Fatalf("link = %d %s", resp.Code, resp.Body.String())
	}
	var got serviceMetricsBody
	resp := performJSONRequest(t, server, http.MethodGet, metricsHistoryPath(id), nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("metrics = %d %s", resp.Code, resp.Body.String())
	}
	decodeResponse(t, resp, &got)
	if got.Data.Resolution != "raw" || len(got.Data.Series) != 1 || got.Data.Series[0].Key != "container:web" || got.Data.Series[0].Source != "os" {
		t.Fatalf("series = %+v", got.Data)
	}
	pts := got.Data.Series[0].Points
	if len(pts) != 1 || pts[0].CPUPercent == nil || *pts[0].CPUPercent != 12.5 || pts[0].MemoryBytes == nil || pts[0].IOReadBps != nil {
		t.Fatalf("points = %+v (an unmeasured field must be absent)", pts)
	}

	// Unlinking hides the series but keeps the samples: linking again brings the same history back.
	performJSONRequest(t, server, http.MethodDelete, "/v1/services/"+id+"/links?kind=resource&node_id="+nodeID+"&key=container:web", nil, "")
	var after serviceMetricsBody
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, metricsHistoryPath(id), nil, ""), &after)
	if len(after.Data.Series) != 0 {
		t.Fatalf("after unlink: %+v", after.Data.Series)
	}
	performJSONRequest(t, server, http.MethodPost, "/v1/services/"+id+"/links", link, "")
	var again serviceMetricsBody
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, metricsHistoryPath(id), nil, ""), &again)
	if len(again.Data.Series) != 1 || len(again.Data.Series[0].Points) != 1 {
		t.Fatalf("relinked history = %+v", again.Data.Series)
	}
}

func TestServiceMetricsErrors(t *testing.T) {
	server, _ := serverWithMetrics(t)
	id := createServiceVia(t, server, gin.H{"name": "Checkout"})
	for name, tc := range map[string]struct {
		path string
		want int
	}{
		"missing service":  {"/v1/services/svc-missing/metrics", http.StatusNotFound},
		"bad from":         {"/v1/services/" + id + "/metrics?from=yesterday", http.StatusBadRequest},
		"from after to":    {"/v1/services/" + id + "/metrics?from=-1h&to=-2h", http.StatusBadRequest},
		"range too long":   {"/v1/services/" + id + "/metrics?from=-100d", http.StatusBadRequest},
		"bad max_points":   {"/v1/services/" + id + "/metrics?max_points=zero", http.StatusBadRequest},
		"default window":   {"/v1/services/" + id + "/metrics", http.StatusOK},
		"relative offsets": {"/v1/services/" + id + "/metrics?from=-6h&to=-1h&max_points=100", http.StatusOK},
	} {
		if resp := performJSONRequest(t, server, http.MethodGet, tc.path, nil, ""); resp.Code != tc.want {
			t.Errorf("%s: status = %d, want %d (%s)", name, resp.Code, tc.want, resp.Body.String())
		}
	}
}

func TestServiceMetricsIs503WithoutAMetricStore(t *testing.T) {
	server := setupTestServer(t)
	id := createServiceVia(t, server, gin.H{"name": "Checkout"})
	if resp := performJSONRequest(t, server, http.MethodGet, "/v1/services/"+id+"/metrics", nil, ""); resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", resp.Code)
	}
	// Reports still work and store the latest values with no store attached.
	node := registerTestNode(t, server)
	reportWithAppResources(t, server, node.Data.NodeID, node.Data.Token, map[string]any{"series": []map[string]any{{"kind": "container", "key": "c", "name": "c", "cpu_percent": 1.0}}})
}

// metricsHistoryPath reads the last hour with an upper bound a little ahead of now. Samples are stored
// at the start of their minute and the range is half-open, so a bound of "now" would leave out the
// sample of the current minute whenever the test runs in the first second of a minute.
func metricsHistoryPath(serviceID string) string {
	return "/v1/services/" + serviceID + "/metrics?from=-1h&to=" + time.Now().UTC().Add(2*time.Minute).Format(time.RFC3339)
}
