package api

import (
	"context"
	"net"
	"net/http"
	"orion/core/internal/metricstore"
	"orion/core/internal/tracestore"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	collogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collmetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// otlpGRPCClient serves OTLP/gRPC for the server on an in-memory listener and returns a client connected to
// it. The receiver is stopped, and waited for, when the test ends.
func otlpGRPCClient(t *testing.T, server *Server) *grpc.ClientConn {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.ServeOTLPGRPC(ctx, listener) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn.Close()
		cancel()
		if err := <-done; err != nil {
			t.Errorf("receiver stopped with %v", err)
		}
	})
	return conn
}

// withKey sends the ingest key the way an OpenTelemetry exporter's headers setting does.
func withKey(key string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+key)
}

func wantCode(t *testing.T, what string, err error, want codes.Code) *status.Status {
	t.Helper()
	st, _ := status.FromError(err)
	if st.Code() != want {
		t.Fatalf("%s: code = %v (%v), want %v", what, st.Code(), err, want)
	}
	return st
}

func TestOTLPGRPCLogsAreStoredAndPartialSuccessIsReported(t *testing.T) {
	server := setupLogsServer(t, nil)
	appID, key := createAppWithKey(t, server, "otel-app")
	client := collogs.NewLogsServiceClient(otlpGRPCClient(t, server))

	// The shared fixture holds two good records and one empty one, which is refused on its own.
	resp, err := client.Export(withKey(key), otlpRequest(time.Now().UTC().Add(-time.Minute)))
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetPartialSuccess().GetRejectedLogRecords() != 1 || resp.GetPartialSuccess().GetErrorMessage() == "" {
		t.Fatalf("partial_success = %+v, want 1 rejected record with a reason", resp.GetPartialSuccess())
	}
	entries := searchLogsAPI(t, server, "app_id="+appID+"&limit=10").Data.Entries
	if len(entries) != 2 {
		t.Fatalf("stored %d entries, want the 2 valid ones", len(entries))
	}
	for _, e := range entries {
		if e.Severity == "error" && e.Attributes["password"] != "[redacted]" {
			t.Fatalf("password = %v, want redacted: gRPC goes through the same redaction as HTTP", e.Attributes["password"])
		}
	}

	// A clean request has an empty response, as on HTTP.
	clean := otlpRequest(time.Now().UTC())
	clean.ResourceLogs[0].ScopeLogs[0].LogRecords = clean.ResourceLogs[0].ScopeLogs[0].LogRecords[:1]
	if resp, err := client.Export(withKey(key), clean); err != nil || resp.GetPartialSuccess() != nil {
		t.Fatalf("clean export = %+v, %v; want an empty response", resp, err)
	}
}

func TestOTLPGRPCMetricsShowUpOnTheServiceOfTheApp(t *testing.T) {
	server, store := metricsServer(t)
	appID, key := createAppWithKey(t, server, "checkout-api")
	svcID := createServiceVia(t, server, gin.H{"name": "Checkout"})
	performJSONRequest(t, server, http.MethodPost, "/v1/services/"+svcID+"/links", gin.H{"kind": "app", "id": appID}, "")
	client := collmetrics.NewMetricsServiceClient(otlpGRPCClient(t, server))

	// A reading from a minute ago, so the test does not care where in the minute it runs; the history is
	// read with a "to" ahead of now (a sample sits at the start of its minute and the range is half open).
	resp, err := client.Export(withKey(key), gaugeRequest(time.Now().UTC().Add(-time.Minute), 2))
	if err != nil || resp.GetPartialSuccess() != nil {
		t.Fatalf("export = %+v, %v", resp, err)
	}
	store.Flush()
	var got struct {
		Data struct {
			Series []struct {
				Source string `json:"source"`
				AppID  string `json:"app_id"`
				Metric string `json:"metric"`
				Points []struct {
					Value *float64 `json:"value"`
				} `json:"points"`
			} `json:"series"`
		} `json:"data"`
	}
	r := performJSONRequest(t, server, http.MethodGet, metricsHistoryPath(svcID), nil, "")
	if r.Code != http.StatusOK {
		t.Fatalf("history status = %d %s", r.Code, r.Body.String())
	}
	decodeResponse(t, r, &got)
	if len(got.Data.Series) != 2 || got.Data.Series[0].Source != "otlp" || got.Data.Series[0].AppID != appID ||
		got.Data.Series[0].Metric != "app.metric.00" || len(got.Data.Series[0].Points) != 1 || *got.Data.Series[0].Points[0].Value != 0.5 {
		t.Fatalf("series = %+v", got.Data.Series)
	}

	// Series over the app's cap are refused in partial_success and the rest is stored.
	over, err := client.Export(withKey(key), gaugeRequest(time.Now().UTC(), metricstore.DefaultSeriesCapPerApp+5))
	if err != nil {
		t.Fatal(err)
	}
	if over.GetPartialSuccess().GetRejectedDataPoints() != 5 || !strings.Contains(over.GetPartialSuccess().GetErrorMessage(), "series limit") {
		t.Fatalf("partial_success = %+v, want the 5 series over the cap with a reason", over.GetPartialSuccess())
	}
}

func TestOTLPGRPCTracesAreStoredRedactedAndPartialSuccessIsReported(t *testing.T) {
	server, store := tracesServer(t, nil, 0)
	appID, key := createAppWithKey(t, server, "checkout-api")
	client := coltrace.NewTraceServiceClient(otlpGRPCClient(t, server))

	now := time.Now().UTC().Add(-time.Minute)
	root := otlpSpan(1, 1, 0, "POST /pay", now, 120*time.Millisecond, false)
	root.Attributes = []*commonpb.KeyValue{strKV("user.password", "hunter2"), strKV("http.route", "/pay")}
	bad := otlpSpan(1, 2, 1, "bad", now, time.Millisecond, false)
	bad.TraceId = []byte{1, 2, 3} // not 16 bytes
	resp, err := client.Export(withKey(key), otlpTraceRequest("checkout-api", root, otlpSpan(1, 3, 1, "child", now, time.Millisecond, false), bad))
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetPartialSuccess().GetRejectedSpans() != 1 || !strings.Contains(resp.GetPartialSuccess().GetErrorMessage(), "span 2") {
		t.Fatalf("partial_success = %+v, want span 2 refused with a reason", resp.GetPartialSuccess())
	}
	store.Flush(5 * time.Second)

	list := getTraces(t, server, "app_id="+appID, http.StatusOK)
	if len(list.Data.Traces) != 1 || list.Data.Traces[0].RootName != "POST /pay" || list.Data.Traces[0].SpanCount != 2 {
		t.Fatalf("traces = %+v, want one trace of the 2 good spans", list.Data.Traces)
	}
	detail := performJSONRequest(t, server, http.MethodGet, "/v1/traces/"+traceHex(1), nil, "")
	if body := detail.Body.String(); strings.Contains(body, "hunter2") || !strings.Contains(body, "[redacted]") || !strings.Contains(body, "/pay") {
		t.Fatalf("trace = %s: the secret must be redacted and other attributes kept", body)
	}
}

func TestOTLPGRPCNeedsAValidIngestKeyOnEverySignal(t *testing.T) {
	server, store := tracesServer(t, nil, 0)
	_, key := createAppWithKey(t, server, "checkout-api")
	conn := otlpGRPCClient(t, server)
	logsClient, metricsClient, traceClient := collogs.NewLogsServiceClient(conn), collmetrics.NewMetricsServiceClient(conn), coltrace.NewTraceServiceClient(conn)
	span := otlpTraceRequest("svc", otlpSpan(1, 1, 0, "root", time.Now(), time.Millisecond, false))

	for name, ctx := range map[string]context.Context{
		"no key":             context.Background(),
		"unknown key":        withKey("orion_ik_0000000000000000000000000000000000000000"),
		"not an ingest key":  withKey("abc"),
		"empty bearer value": metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "),
	} {
		_, err := logsClient.Export(ctx, otlpRequest(time.Now()))
		wantCode(t, name+" (logs)", err, codes.Unauthenticated)
		_, err = metricsClient.Export(ctx, gaugeRequest(time.Now(), 1))
		wantCode(t, name+" (metrics)", err, codes.Unauthenticated)
		_, err = traceClient.Export(ctx, span)
		wantCode(t, name+" (traces)", err, codes.Unauthenticated)
	}
	if _, err := traceClient.Export(withKey(key), span); err != nil {
		t.Fatalf("valid key: %v", err)
	}
	store.Flush(5 * time.Second)
	if list := getTraces(t, server, "", http.StatusOK); len(list.Data.Traces) != 1 {
		t.Fatalf("traces = %d, want only the authenticated export stored", len(list.Data.Traces))
	}
}

// A call without a key is refused before its message is read, so nothing it carries is decoded.
func TestOTLPGRPCRefusesUnauthenticatedCallsBeforeReadingTheMessage(t *testing.T) {
	server := setupLogsServer(t, nil)
	conn := otlpGRPCClient(t, server)
	// A body that would fail to decode: the answer is still Unauthenticated, not InvalidArgument.
	err := conn.Invoke(context.Background(), "/opentelemetry.proto.collector.logs.v1.LogsService/Export", []byte{0xff, 0xff, 0xff, 0xff, 0x01}, &collogs.ExportLogsServiceResponse{}, grpc.ForceCodec(rawProtoCodec{}))
	wantCode(t, "garbage without a key", err, codes.Unauthenticated)
}

func TestOTLPGRPCAnswersUnavailableWhenTheSignalIsOff(t *testing.T) {
	server := setupLogsServer(t, nil) // logs only: no metrics store, no traces store
	_, key := createAppWithKey(t, server, "checkout-api")
	conn := otlpGRPCClient(t, server)

	_, err := coltrace.NewTraceServiceClient(conn).Export(withKey(key), otlpTraceRequest("svc", otlpSpan(1, 1, 0, "root", time.Now(), time.Millisecond, false)))
	wantCode(t, "traces off", err, codes.Unavailable)
	_, err = collmetrics.NewMetricsServiceClient(conn).Export(withKey(key), gaugeRequest(time.Now(), 1))
	wantCode(t, "metrics off", err, codes.Unavailable)
	// Without a valid key the answer is still Unauthenticated, not a hint that the signal is off.
	_, err = coltrace.NewTraceServiceClient(conn).Export(context.Background(), otlpTraceRequest("svc"))
	wantCode(t, "traces off, no key", err, codes.Unauthenticated)
}

func TestOTLPGRPCRefusesRequestsThatCannotBeStored(t *testing.T) {
	server, _ := tracesServer(t, nil, 0)
	_, key := createAppWithKey(t, server, "checkout-api")
	conn := otlpGRPCClient(t, server)

	// Bytes that are not a protobuf message: the client's mistake, not an Internal error.
	for _, method := range []string{
		"/opentelemetry.proto.collector.logs.v1.LogsService/Export",
		"/opentelemetry.proto.collector.metrics.v1.MetricsService/Export",
		"/opentelemetry.proto.collector.trace.v1.TraceService/Export",
	} {
		err := conn.Invoke(withKey(key), method, []byte{0xff, 0xff, 0xff, 0xff, 0x01}, &coltrace.ExportTraceServiceResponse{}, grpc.ForceCodec(rawProtoCodec{}))
		wantCode(t, "malformed "+method, err, codes.InvalidArgument)
	}

	// More spans, data points or records than one request may carry.
	now := time.Now()
	spans := make([]*tracepb.Span, tracestore.MaxSpansPerRequest+1)
	for i := range spans {
		spans[i] = otlpSpan(1, 1, 0, "s", now, time.Millisecond, false)
	}
	_, err := coltrace.NewTraceServiceClient(conn).Export(withKey(key), otlpTraceRequest("svc", spans...))
	if st := wantCode(t, "too many spans", err, codes.InvalidArgument); !strings.Contains(st.Message(), "Too many spans") {
		t.Fatalf("message = %q", st.Message())
	}
	metricsApp, metricsStore := metricsServer(t)
	_, metricsKey := createAppWithKey(t, metricsApp, "points")
	_, err = collmetrics.NewMetricsServiceClient(otlpGRPCClient(t, metricsApp)).Export(withKey(metricsKey), gaugeRequest(now, metricstore.MaxPointsPerRequest+1))
	wantCode(t, "too many data points", err, codes.InvalidArgument)
	metricsStore.Flush()
	logs := otlpRequest(now)
	one := logs.ResourceLogs[0].ScopeLogs[0].LogRecords[0]
	logs.ResourceLogs[0].ScopeLogs[0].LogRecords = nil
	for i := 0; i < 10001; i++ {
		logs.ResourceLogs[0].ScopeLogs[0].LogRecords = append(logs.ResourceLogs[0].ScopeLogs[0].LogRecords, one)
	}
	_, err = collogs.NewLogsServiceClient(conn).Export(withKey(key), logs)
	wantCode(t, "too many log records", err, codes.InvalidArgument)
}
