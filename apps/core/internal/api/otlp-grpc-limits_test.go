package api

import (
	"bytes"
	"context"
	"net"
	"orion/core/internal/config"
	"strconv"
	"sync"
	"testing"
	"time"

	collogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

// rawProtoCodec sends the bytes it is given as the message, so a test can put a body on the wire that no
// generated client would produce.
type rawProtoCodec struct{}

func (rawProtoCodec) Marshal(v any) ([]byte, error) { return v.([]byte), nil }
func (rawProtoCodec) Unmarshal(data []byte, v any) error {
	return proto.Unmarshal(data, v.(proto.Message))
}
func (rawProtoCodec) Name() string { return "proto" }

func spans(n int) []*tracepb.Span {
	out := make([]*tracepb.Span, n)
	for i := range out {
		out[i] = otlpSpan(1, i+1, 0, "s", time.Now(), time.Millisecond, false)
	}
	return out
}

// retryHints reads what a client needs to back off: the retry-after trailer and the RetryInfo detail.
func retryHints(t *testing.T, err error, trailer metadata.MD) (seconds int, delay time.Duration) {
	t.Helper()
	values := trailer.Get("retry-after")
	if len(values) != 1 {
		t.Fatalf("retry-after trailer = %v, want one value", values)
	}
	seconds, convErr := strconv.Atoi(values[0])
	if convErr != nil || seconds < 1 {
		t.Fatalf("retry-after = %q, want whole seconds of at least 1", values[0])
	}
	st, _ := status.FromError(err)
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.RetryInfo); ok {
			return seconds, info.GetRetryDelay().AsDuration()
		}
	}
	t.Fatalf("error %v has no RetryInfo detail", err)
	return 0, 0
}

func TestOTLPGRPCRateLimitSharesTheAppBudgetAndSaysWhenToRetry(t *testing.T) {
	server, store := tracesServer(t, &config.Config{LogsAppRatePerSecond: 5}, 0)
	_, key := createAppWithKey(t, server, "noisy")
	conn := otlpGRPCClient(t, server)
	traceClient := coltrace.NewTraceServiceClient(conn)

	if _, err := traceClient.Export(withKey(key), otlpTraceRequest("svc", spans(10)...)); err != nil {
		t.Fatalf("first burst: %v", err)
	}
	var trailer metadata.MD
	_, err := traceClient.Export(withKey(key), otlpTraceRequest("svc", spans(10)...), grpc.Trailer(&trailer))
	st := wantCode(t, "second burst", err, codes.ResourceExhausted)
	if len(st.Details()) != 1 {
		t.Fatalf("details = %v, want a RetryInfo", st.Details())
	}
	seconds, delay := retryHints(t, err, trailer)
	if delay != time.Duration(seconds)*time.Second {
		t.Fatalf("RetryInfo delay = %v, retry-after = %ds: they must agree", delay, seconds)
	}
	// The limiter is the one logs use: a log record right after is refused too.
	_, err = collogs.NewLogsServiceClient(conn).Export(withKey(key), otlpRequest(time.Now()))
	wantCode(t, "logs after the span burst", err, codes.ResourceExhausted)
	store.Flush(5 * time.Second)
}

func TestOTLPGRPCAnswersResourceExhaustedWhenTheWriteQueueIsFull(t *testing.T) {
	server, _ := tracesServer(t, nil, 2)
	_, key := createAppWithKey(t, server, "checkout-api")
	var trailer metadata.MD
	_, err := coltrace.NewTraceServiceClient(otlpGRPCClient(t, server)).Export(withKey(key), otlpTraceRequest("svc", spans(3)...), grpc.Trailer(&trailer))
	wantCode(t, "queue full", err, codes.ResourceExhausted)
	if seconds, delay := retryHints(t, err, trailer); seconds != 1 || delay != time.Second {
		t.Fatalf("retry-after = %ds, delay = %v, want 1s as on HTTP", seconds, delay)
	}
}

// Every failure of the shared ingest path has a status, and only the ones worth retrying say when.
func TestOTLPGRPCStatusMapping(t *testing.T) {
	cases := []struct {
		name    string
		failure ingestFailure
		code    codes.Code
		retry   bool
	}{
		{"no key", ingestFailure{kind: failUnauthenticated}, codes.Unauthenticated, false},
		{"signal off", ingestFailure{kind: failUnavailable}, codes.Unavailable, false},
		{"too many", ingestFailure{kind: failTooMany}, codes.InvalidArgument, false},
		{"invalid", ingestFailure{kind: failInvalid}, codes.InvalidArgument, false},
		{"rate limited", ingestFailure{kind: failRateLimited, retryAfter: 1500 * time.Millisecond}, codes.ResourceExhausted, true},
		{"queue full", ingestFailure{kind: failQueueFull, retryAfter: time.Second}, codes.ResourceExhausted, true},
		{"storage full", ingestFailure{kind: failStorageFull, retryAfter: 5 * time.Second}, codes.Unavailable, true},
		{"internal", ingestFailure{kind: failInternal, message: "boom", err: context.Canceled}, codes.Internal, false},
	}
	for _, tc := range cases {
		err := otlpGRPCStatus(context.Background(), &tc.failure, "spans")
		st, _ := status.FromError(err)
		if st.Code() != tc.code {
			t.Errorf("%s: code = %v, want %v", tc.name, st.Code(), tc.code)
		}
		if got := len(st.Details()) > 0; got != tc.retry {
			t.Errorf("%s: has RetryInfo = %v, want %v", tc.name, got, tc.retry)
		}
		if tc.name == "internal" && st.Message() != "Internal error" {
			t.Errorf("internal message = %q: details of a server fault must not reach the client", st.Message())
		}
	}
}

func TestOTLPGRPCAcceptsGzipAndEnforcesTheMessageCapAfterDecompression(t *testing.T) {
	server, store := tracesServer(t, nil, 0)
	_, key := createAppWithKey(t, server, "checkout-api")
	conn := otlpGRPCClient(t, server)

	if _, err := coltrace.NewTraceServiceClient(conn).Export(withKey(key), otlpTraceRequest("svc", spans(20)...), grpc.UseCompressor(gzip.Name)); err != nil {
		t.Fatalf("gzip export: %v", err)
	}
	store.Flush(5 * time.Second)
	if list := getTraces(t, server, "", 200); len(list.Data.Traces) != 1 || list.Data.Traces[0].SpanCount != 20 {
		t.Fatalf("traces = %+v, want the 20 spans of the gzip export", list.Data.Traces)
	}

	logsClient := collogs.NewLogsServiceClient(conn)
	withBody := func(size int) *collogs.ExportLogsServiceRequest {
		request := otlpRequest(time.Now())
		record := request.ResourceLogs[0].ScopeLogs[0].LogRecords[0]
		record.Body = &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: string(bytes.Repeat([]byte("a"), size))}}
		request.ResourceLogs[0].ScopeLogs[0].LogRecords = []*logspb.LogRecord{record}
		return request
	}
	// 4 MiB is under the 5 MiB cap that OTLP/HTTP has: read, and the one oversized record is refused on
	// its own (64 KiB per entry) in partial_success.
	resp, err := logsClient.Export(withKey(key), withBody(4<<20))
	if err != nil || resp.GetPartialSuccess().GetRejectedLogRecords() != 1 {
		t.Fatalf("4 MiB export = %+v, %v; want accepted with the record refused", resp, err)
	}
	// 6 MiB is over it, plain or gzipped (a few KiB on the wire, 6 MiB once decompressed).
	for name, options := range map[string][]grpc.CallOption{"plain": nil, "gzip bomb": {grpc.UseCompressor(gzip.Name)}} {
		_, err := logsClient.Export(withKey(key), withBody(6<<20), options...)
		st := wantCode(t, name+" over 5 MiB", err, codes.ResourceExhausted)
		if len(st.Details()) != 0 {
			t.Fatalf("%s: details = %v: a message that is too large must not invite a retry", name, st.Details())
		}
	}
}

func TestOTLPGRPCServesManyClientsAtOnce(t *testing.T) {
	server, store := tracesServer(t, nil, 0)
	_, key := createAppWithKey(t, server, "checkout-api")
	client := coltrace.NewTraceServiceClient(otlpGRPCClient(t, server))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				if _, err := client.Export(withKey(key), otlpTraceRequest("svc", spans(3)...)); err != nil {
					t.Errorf("export: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	store.Flush(10 * time.Second)
}

func TestOTLPGRPCIsOffByDefault(t *testing.T) {
	if got := config.Load().OTLPGRPCAddr; got != "" {
		t.Fatalf("default ORION_OTLP_GRPC_ADDR = %q, want empty (receiver off)", got)
	}
	server := setupLogsServer(t, nil)
	addr, done, err := server.StartOTLPGRPC(context.Background(), "")
	if addr != nil || done != nil || err != nil {
		t.Fatalf("StartOTLPGRPC(\"\") = %v, %v, %v; want nothing started", addr, done, err)
	}
	t.Setenv("ORION_OTLP_GRPC_ADDR", " :4317 ")
	if got := config.Load().OTLPGRPCAddr; got != ":4317" {
		t.Fatalf("ORION_OTLP_GRPC_ADDR = %q, want :4317", got)
	}
}

func TestOTLPGRPCServesOverTCPAndStopsGracefully(t *testing.T) {
	server, store := tracesServer(t, nil, 0)
	_, key := createAppWithKey(t, server, "checkout-api")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr, done, err := server.StartOTLPGRPC(ctx, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.NewClient(addr.String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := coltrace.NewTraceServiceClient(conn).Export(withKey(key), otlpTraceRequest("svc", spans(2)...)); err != nil {
		t.Fatalf("export over TCP: %v", err)
	}
	store.Flush(5 * time.Second)

	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the receiver did not stop after its context was cancelled")
	}
	if c, err := net.DialTimeout("tcp", addr.String(), time.Second); err == nil {
		c.Close()
		t.Fatal("the port still accepts connections after shutdown")
	}
	// A listener that cannot be opened is reported to the caller (Core refuses to start).
	if _, _, err := server.StartOTLPGRPC(context.Background(), "256.0.0.1:1"); err == nil {
		t.Fatal("an unusable address must be an error")
	}
}

func TestOTLPGRPCShutdownCutsACallThatNeverFinishes(t *testing.T) {
	grace := otlpGRPCShutdownGrace
	otlpGRPCShutdownGrace = 300 * time.Millisecond
	t.Cleanup(func() { otlpGRPCShutdownGrace = grace })

	server, _ := tracesServer(t, nil, 0)
	_, key := createAppWithKey(t, server, "checkout-api")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr, done, err := server.StartOTLPGRPC(ctx, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.NewClient(addr.String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// Open a call and never send its message: graceful stop waits for it, then the grace period ends.
	stream, err := conn.NewStream(withKey(key), &grpc.StreamDesc{StreamName: "Export"}, "/opentelemetry.proto.collector.trace.v1.TraceService/Export")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // let the server see the call

	started := time.Now()
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("a stuck call kept the receiver from stopping")
	}
	if waited := time.Since(started); waited < 250*time.Millisecond {
		t.Fatalf("stopped after %v: the call should have been given its grace period", waited)
	}
	if err := stream.RecvMsg(&coltrace.ExportTraceServiceResponse{}); err == nil {
		t.Fatal("the cut call should fail")
	}
}

func TestLimitListenerHoldsConnectionsAtTheCapAndWakesOnClose(t *testing.T) {
	inner := bufconn.Listen(1 << 10)
	limited := newLimitListener(inner, 2)
	for i := 0; i < 3; i++ {
		go func() { inner.Dial() }()
	}
	var held []net.Conn
	for i := 0; i < 2; i++ {
		c, err := limited.Accept()
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, c)
	}
	accepted := make(chan error, 1)
	go func() {
		c, err := limited.Accept()
		if c != nil {
			c.Close()
		}
		accepted <- err
	}()
	select {
	case <-accepted:
		t.Fatal("a third connection was accepted over the cap of 2")
	case <-time.After(100 * time.Millisecond):
	}
	held[0].Close() // frees a slot, the waiting Accept takes the third
	select {
	case err := <-accepted:
		if err != nil {
			t.Fatalf("third accept: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("closing a connection did not free a slot")
	}

	// A server that is stopping must not hang on an Accept that waits for a slot.
	held[1].Close()
	blocked := newLimitListener(bufconn.Listen(1<<10), 1)
	blocked.slots <- struct{}{}
	result := make(chan error, 1)
	go func() { _, err := blocked.Accept(); result <- err }()
	time.Sleep(50 * time.Millisecond)
	blocked.Close()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("Accept after Close should fail")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not wake an Accept waiting for a slot")
	}
}
