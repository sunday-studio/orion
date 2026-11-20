package metricstore

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// otlpJSONMetrics is an ExportMetricsServiceRequest as an SDK or the Collector writes it with
// the http/json protocol: a monotonic delta sum whose data point carries an exemplar with hex
// ids, and a gauge. Nanosecond times and int values are strings, enums are names or numbers.
func otlpJSONMetrics(at time.Time) string {
	return fmt.Sprintf(`{
  "resourceMetrics": [{
    "resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "checkout-api"}}]},
    "scopeMetrics": [{
      "scope": {"name": "http-server", "version": "1.2.3"},
      "metrics": [{
        "name": "http.server.requests",
        "unit": "1",
        "sum": {
          "dataPoints": [{
            "attributes": [{"key": "route", "value": {"stringValue": "/pay"}}],
            "startTimeUnixNano": "%[1]d",
            "timeUnixNano": "%[2]d",
            "asInt": "4",
            "exemplars": [{
              "filteredAttributes": [{"key": "user", "value": {"stringValue": "u1"}}],
              "timeUnixNano": "%[2]d",
              "asDouble": 0.5,
              "spanId": "eee19b7ec3c1b174",
              "traceId": "5b8efff798038103d269b633813fc60c"
            }]
          }],
          "aggregationTemporality": "AGGREGATION_TEMPORALITY_DELTA",
          "isMonotonic": true
        }
      }, {
        "name": "process.memory.usage",
        "unit": "By",
        "gauge": {"dataPoints": [{"timeUnixNano": "%[2]d", "asDouble": 1048576}]}
      }]
    }]
  }]
}`, at.Add(-time.Minute).UnixNano(), at.UnixNano())
}

func TestDecodeOTLPMetricsReadsHexExemplarIds(t *testing.T) {
	now := time.Now().UTC()
	request, format, err := DecodeOTLPMetrics("application/json", []byte(otlpJSONMetrics(now)))
	if err != nil || format != OTLPJSON {
		t.Fatalf("format = %v, err = %v", format, err)
	}
	metrics := request.GetResourceMetrics()[0].GetScopeMetrics()[0].GetMetrics()
	exemplar := metrics[0].GetSum().GetDataPoints()[0].GetExemplars()[0]
	if got := hex.EncodeToString(exemplar.GetTraceId()); got != "5b8efff798038103d269b633813fc60c" {
		t.Fatalf("exemplar trace id = %s", got)
	}
	if got := hex.EncodeToString(exemplar.GetSpanId()); got != "eee19b7ec3c1b174" {
		t.Fatalf("exemplar span id = %s", got)
	}
}

func TestDecodeOTLPMetricsKeepsReadingBase64ExemplarIds(t *testing.T) {
	now := time.Now().UTC()
	body := strings.NewReplacer("5b8efff798038103d269b633813fc60c", "W47/95gDgQPSabYzgT/GDA==", "eee19b7ec3c1b174", "7uGbfsPBsXQ=").Replace(otlpJSONMetrics(now))
	request, _, err := DecodeOTLPMetrics("application/json", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	exemplar := request.GetResourceMetrics()[0].GetScopeMetrics()[0].GetMetrics()[0].GetSum().GetDataPoints()[0].GetExemplars()[0]
	if hex.EncodeToString(exemplar.GetTraceId()) != "5b8efff798038103d269b633813fc60c" || hex.EncodeToString(exemplar.GetSpanId()) != "eee19b7ec3c1b174" {
		t.Fatalf("exemplar = %+v", exemplar)
	}
}

func TestDecodeOTLPMetricsRejectsIdsThatAreNeitherHexNorBase64(t *testing.T) {
	body := strings.Replace(otlpJSONMetrics(time.Now().UTC()), "eee19b7ec3c1b174", "not-an-id!!!!!!!", 1)
	if _, _, err := DecodeOTLPMetrics("application/json", []byte(body)); !errors.Is(err, ErrInvalidBody) {
		t.Fatalf("err = %v, want ErrInvalidBody", err)
	}
}

func TestParseOTLPMetricsAcceptsTheSpecShapedJSONBody(t *testing.T) {
	now := time.Now().UTC()
	parsed, format, err := ParseOTLPMetrics("application/json", []byte(otlpJSONMetrics(now)), now)
	if err != nil || format != OTLPJSON || parsed.Rejected != 0 || len(parsed.Points) != 2 {
		t.Fatalf("parsed = %+v, format = %v, err = %v", parsed, format, err)
	}
	sum, gauge := parsed.Points[0].Sample, parsed.Points[1].Sample
	if sum.Metric != "http.server.requests" || sum.Kind != "sum" || *sum.Values.Value != 4 || gauge.Kind != "gauge" || *gauge.Values.Value != 1048576 {
		t.Fatalf("samples = %+v %+v", sum, gauge)
	}
}
