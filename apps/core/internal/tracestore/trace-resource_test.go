package tracestore

import (
	"fmt"
	"strings"
	"testing"
	"time"

	coltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// heavyResource is a resource whose attributes add up to about 500 KB: 124 filler attributes of 4000
// bytes, with the identifying ones deliberately placed last and with names that sort last.
func heavyResource() *resourcepb.Resource {
	var attrs []*commonpb.KeyValue
	for i := 0; i < 124; i++ {
		attrs = append(attrs, strKV(fmt.Sprintf("filler.%03d", i), strings.Repeat("f", 4000)))
	}
	attrs = append(attrs,
		strKV("service.name", "checkout-api"), strKV("service.version", "9.9.9"),
		strKV("zzz.host.name", "last"), strKV("host.name", "web-1"))
	return &resourcepb.Resource{Attributes: attrs}
}

func heavyRequest(spanCount int) *coltrace.ExportTraceServiceRequest {
	spans := make([]*tracepb.Span, spanCount)
	for i := range spans {
		spans[i] = okSpan(i + 1)
	}
	return &coltrace.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		Resource:   heavyResource(),
		ScopeSpans: []*tracepb.ScopeSpans{{Spans: spans}},
	}}}
}

func parseRequest(t *testing.T, req *coltrace.ExportTraceServiceRequest) Parsed {
	t.Helper()
	parsed, _, err := ParseOTLP("application/x-protobuf", encodings(t, req)["application/x-protobuf"], epoch)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestHugeResourceIsBoundedOnEverySpan(t *testing.T) {
	const spanCount = 200
	parsed := parseRequest(t, heavyRequest(spanCount))
	if len(parsed.Errors) != 0 || len(parsed.Spans) != spanCount {
		t.Fatalf("parsed %d spans, errors %+v", len(parsed.Spans), parsed.Errors)
	}
	s := openTestStore(t, 0)
	for i := range parsed.Spans {
		parsed.Spans[i].AppID = "app"
	}
	s.mustWrite(t, parsed.Spans...)

	var stored struct{ Rows, ResourceBytes, MaxResource, TotalBytes int64 }
	if err := s.DB().Raw(`SELECT COUNT(*) AS rows, SUM(LENGTH(CAST(resource AS BLOB))) AS resource_bytes, MAX(LENGTH(CAST(resource AS BLOB))) AS max_resource,
		SUM(LENGTH(CAST(resource AS BLOB)) + LENGTH(CAST(attributes AS BLOB)) + LENGTH(CAST(events AS BLOB)) + LENGTH(CAST(links AS BLOB))) AS total_bytes FROM spans`).Scan(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Rows != spanCount || stored.MaxResource > MaxResourceBytes {
		t.Fatalf("stored %d rows, largest resource %d bytes, cap %d", stored.Rows, stored.MaxResource, MaxResourceBytes)
	}
	// 200 spans of one ~500 KB resource used to store ~100 MB. Now each span carries at most the cap.
	if limit := int64(spanCount) * (MaxResourceBytes + 2<<10); stored.TotalBytes > limit {
		t.Fatalf("stored %d bytes of json for %d tiny spans, want at most %d", stored.TotalBytes, spanCount, limit)
	}
	if _, bytes, _ := s.CountForApp("app"); bytes < stored.ResourceBytes || bytes > stored.TotalBytes+int64(spanCount)*1024 {
		t.Fatalf("CountForApp bytes = %d, resource alone is %d and all json %d: the cap would see the real size", bytes, stored.ResourceBytes, stored.TotalBytes)
	}
}

func TestSizeEqualsTheStoredColumnBytesPlusAConstant(t *testing.T) {
	parsed := parseRequest(t, heavyRequest(5))
	// An event and a link make sure those columns are part of the size too.
	req := heavyRequest(5)
	req.ResourceSpans[0].ScopeSpans[0].Spans[0].Events = []*tracepb.Span_Event{{Name: "boom", Attributes: []*commonpb.KeyValue{strKV("k", "v")}}}
	req.ResourceSpans[0].ScopeSpans[0].Spans[0].Links = []*tracepb.Span_Link{{TraceId: rawID(3, 16), SpanId: rawID(4, 8)}}
	req.ResourceSpans[0].ScopeSpans[0].Spans[0].Attributes = []*commonpb.KeyValue{strKV("http.method", "GET")}
	parsed = parseRequest(t, req)
	s := openTestStore(t, 0)
	for i := range parsed.Spans {
		parsed.Spans[i].AppID = "app"
	}
	s.mustWrite(t, parsed.Spans...)

	var rows []struct {
		Size, Columns int64
	}
	if err := s.DB().Raw(`SELECT size, LENGTH(CAST(trace_id AS BLOB)) + LENGTH(CAST(span_id AS BLOB)) + LENGTH(CAST(parent_span_id AS BLOB)) + LENGTH(CAST(name AS BLOB)) +
		LENGTH(CAST(status_message AS BLOB)) + LENGTH(CAST(service_name AS BLOB)) + LENGTH(CAST(attributes AS BLOB)) + LENGTH(CAST(resource AS BLOB)) +
		LENGTH(CAST(events AS BLOB)) + LENGTH(CAST(links AS BLOB)) AS columns FROM spans`).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("rows = %d", len(rows))
	}
	for _, r := range rows {
		if r.Size != r.Columns+spanRowOverhead {
			t.Fatalf("size %d, text columns %d: size must be the columns plus the %d byte constant", r.Size, r.Columns, spanRowOverhead)
		}
	}
	// The constant has to cover the app id and the integer columns the formula leaves out.
	if spanRowOverhead < len("app")+7*8+16 {
		t.Fatalf("spanRowOverhead %d does not cover the integer columns", spanRowOverhead)
	}
	for _, sp := range parsed.Spans {
		if sp.Size <= 0 {
			t.Fatalf("parse left Size unset: %+v", sp)
		}
	}
}

func TestParsedSizeCountsTheCappedResource(t *testing.T) {
	parsed := parseRequest(t, heavyRequest(1))
	sp := parsed.Spans[0]
	resource := len(encodeJSON(sp.Resource))
	if resource > MaxResourceBytes || resource < MaxResourceBytes/2 {
		t.Fatalf("resource json = %d bytes, want close to the %d cap", resource, MaxResourceBytes)
	}
	if sp.Size < resource {
		t.Fatalf("Size %d does not include the %d resource bytes", sp.Size, resource)
	}
	_, size := encodeRow(sp)
	if size != sp.Size {
		t.Fatalf("parse size %d differs from the writer's %d", sp.Size, size)
	}
}

func TestResourceCapKeepsTheIdentifyingKeysAndCountsTheDrops(t *testing.T) {
	req := heavyRequest(1)
	req.ResourceSpans[0].Resource.Attributes = append(req.ResourceSpans[0].Resource.Attributes,
		strKV("k8s.pod.name", "pod-7"), strKV("cloud.region", "eu-west-1"), strKV("deployment.environment", "prod"),
		strKV("service.namespace", "shop"), strKV("service.instance.id", "i-1"))
	parsed := parseRequest(t, req)
	sp := parsed.Spans[0]
	for key, want := range map[string]string{
		"service.name": "checkout-api", "service.namespace": "shop", "service.version": "9.9.9", "service.instance.id": "i-1",
		"host.name": "web-1", "deployment.environment": "prod", "k8s.pod.name": "pod-7", "cloud.region": "eu-west-1",
	} {
		if sp.Resource[key] != want {
			t.Fatalf("resource[%s] = %v, want %q: identifying keys survive the cap (%d keys kept)", key, sp.Resource[key], want, len(sp.Resource))
		}
	}
	if sp.ServiceName != "checkout-api" {
		t.Fatalf("service name = %q", sp.ServiceName)
	}
	raw := len(req.ResourceSpans[0].Resource.Attributes)
	dropped, _ := sp.Attributes["orion.dropped_resource_attributes"].(int64)
	if dropped == 0 || int(dropped) != raw-len(sp.Resource) {
		t.Fatalf("orion.dropped_resource_attributes = %v, want %d (sent %d, kept %d)", sp.Attributes["orion.dropped_resource_attributes"], raw-len(sp.Resource), raw, len(sp.Resource))
	}
	// After the identifying keys, the rest are kept in sorted order: the kept fillers are a prefix.
	for i := 0; i < 124; i++ {
		_, kept := sp.Resource[fmt.Sprintf("filler.%03d", i)]
		if i > 0 {
			_, prev := sp.Resource[fmt.Sprintf("filler.%03d", i-1)]
			if kept && !prev {
				t.Fatalf("filler.%03d kept but filler.%03d dropped: other keys must go in sorted order", i, i-1)
			}
		}
	}
	if _, ok := sp.Resource["filler.000"]; !ok {
		t.Fatal("the first sorted filler attribute should still fit")
	}
}

func TestSmallResourceIsStoredUntouched(t *testing.T) {
	parsed := parseRequest(t, request(okSpan(1)))
	sp := parsed.Spans[0]
	if len(sp.Resource) != 2 || sp.Attributes["orion.dropped_resource_attributes"] != nil {
		t.Fatalf("a small resource must pass untouched: %+v %+v", sp.Resource, sp.Attributes)
	}
}

func TestSpanOverTheLimitOnceStoredIsRejected(t *testing.T) {
	// 108 attributes of 2400 bytes: just under the limit on the wire, over it once the 8 KiB resource
	// is stored next to them.
	var attrs []*commonpb.KeyValue
	for i := 0; i < 108; i++ {
		attrs = append(attrs, strKV(fmt.Sprintf("big.%03d", i), strings.Repeat("v", 2400)))
	}
	big := okSpan(1)
	big.Attributes = attrs
	req := heavyRequest(0)
	req.ResourceSpans[0].ScopeSpans[0].Spans = []*tracepb.Span{big, okSpan(2)}
	parsed := parseRequest(t, req)
	if len(parsed.Spans) != 1 || len(parsed.Errors) != 1 || parsed.Errors[0].Code != "span_too_large" || parsed.Errors[0].Index != 0 {
		t.Fatalf("spans %d, errors %+v: the stored size, not only the wire size, is limited", len(parsed.Spans), parsed.Errors)
	}
	if parsed.Spans[0].Size > MaxSpanBytes {
		t.Fatalf("an accepted span is %d bytes, over the %d limit", parsed.Spans[0].Size, MaxSpanBytes)
	}
}

func TestInsertStatementsAreBoundedByBytes(t *testing.T) {
	if !fitsChunk(0, 0, 50<<20) {
		t.Fatal("a statement must always take its first row, however large")
	}
	if fitsChunk(1, 7<<20, 2<<20) {
		t.Fatal("a second row that pushes the statement past the byte budget must start a new one")
	}
	if !fitsChunk(1, 4<<20, 2<<20) || fitsChunk(insertChunk, 0, 1) {
		t.Fatal("rows within the byte budget share a statement, up to the row cap")
	}
	if insertChunkBytes > 8<<20 {
		t.Fatalf("insertChunkBytes = %d, want at most 8 MiB", insertChunkBytes)
	}

	// Spans far over one chunk's budget still all get stored, one statement per few spans.
	s := openTestStore(t, 0)
	var spans []Span
	for i := 0; i < 30; i++ {
		sp := mkSpan("app", traceID(i+1), spanID(i+1), "", "big", time.Duration(i)*time.Second, time.Millisecond, StatusUnset)
		sp.StatusMessage = strings.Repeat("m", 1<<20)
		spans = append(spans, sp)
	}
	s.mustWrite(t, spans...)
	count, bytes, err := s.CountForApp("app")
	if err != nil || count != 30 || bytes < 30<<20 {
		t.Fatalf("stored %d spans, %d bytes, err %v", count, bytes, err)
	}
}

func TestSpansQueuedForADeletedAppAreNotWrittenAfterTheDelete(t *testing.T) {
	s := openTestStore(t, 0)
	s.mustWrite(t, mkSpan("gone", traceID(1), spanID(1), "", "before", 0, time.Second, StatusUnset))
	if n, err := s.DeleteApp("gone"); err != nil || n != 1 {
		t.Fatalf("delete = %d, %v", n, err)
	}
	dropped := s.Stats().Dropped
	s.mustWrite(t,
		mkSpan("gone", traceID(2), spanID(2), "", "late", 0, time.Second, StatusUnset),
		mkSpan("kept", traceID(3), spanID(3), "", "other app", 0, time.Second, StatusUnset))
	if n, _, _ := s.CountForApp("gone"); n != 0 {
		t.Fatalf("%d spans of the deleted app were written after the delete", n)
	}
	if n := countRows(t, s, "traces"); n != 1 {
		t.Fatalf("summaries = %d, want only the other app's", n)
	}
	if n, _, _ := s.CountForApp("kept"); n != 1 {
		t.Fatalf("the other app's span is missing: %d", n)
	}
	if got := s.Stats().Dropped - dropped; got != 1 {
		t.Fatalf("dropped grew by %d, want 1", got)
	}
}
