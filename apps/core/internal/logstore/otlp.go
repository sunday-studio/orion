package logstore

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	collogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// OTLPFormat is the wire encoding of an OTLP/HTTP request, which the response mirrors.
type OTLPFormat int

const (
	OTLPJSON OTLPFormat = iota
	OTLPProtobuf
)

// ParseOTLP decodes an OTLP/HTTP ExportLogsServiceRequest (protobuf or JSON) and maps
// every log record onto the native entry shape, so OTLP and native ingest share the
// same normalisation, limits and redaction.
func ParseOTLP(contentType string, body []byte, now time.Time) (Parsed, OTLPFormat, error) {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	request := &collogs.ExportLogsServiceRequest{}
	var format OTLPFormat
	switch mediaType {
	case "application/x-protobuf", "application/protobuf":
		format = OTLPProtobuf
		if err := proto.Unmarshal(body, request); err != nil {
			return Parsed{}, format, fmt.Errorf("%w: %v", ErrInvalidBody, err)
		}
	case "application/json":
		format = OTLPJSON
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(body, request); err != nil {
			return Parsed{}, format, fmt.Errorf("%w: %v", ErrInvalidBody, err)
		}
	default:
		return Parsed{}, OTLPJSON, ErrUnsupportedMedia
	}

	total := 0
	for _, rl := range request.GetResourceLogs() {
		for _, sl := range rl.GetScopeLogs() {
			total += len(sl.GetLogRecords())
		}
	}
	if total > MaxEntriesPerRequest {
		return Parsed{}, format, ErrTooManyEntries
	}

	var result Parsed
	index := 0
	for _, rl := range request.GetResourceLogs() {
		resource := keyValues(rl.GetResource().GetAttributes())
		for _, sl := range rl.GetScopeLogs() {
			scope := sl.GetScope()
			for _, record := range sl.GetLogRecords() {
				object := recordObject(record, resource, scope)
				entry, entryErr := Normalize(object, proto.Size(record), now)
				if entryErr != nil {
					entryErr.Index = index
					result.Errors = append(result.Errors, *entryErr)
				} else {
					result.Entries = append(result.Entries, entry)
				}
				index++
			}
		}
	}
	return result, format, nil
}

// recordObject builds the native envelope for one OTLP record.
func recordObject(record *logspb.LogRecord, resource map[string]any, scope *commonpb.InstrumentationScope) map[string]any {
	attributes := keyValues(record.GetAttributes())
	if attributes == nil {
		attributes = map[string]any{}
	}
	if scope.GetName() != "" {
		attributes["otel.scope.name"] = scope.GetName()
	}
	if scope.GetVersion() != "" {
		attributes["otel.scope.version"] = scope.GetVersion()
	}
	object := map[string]any{"attributes": attributes}
	if len(resource) > 0 {
		object["resource"] = resource
	}

	if body := record.GetBody(); body != nil {
		object["body"] = anyValue(body)
	}
	switch {
	case record.GetTimeUnixNano() != 0:
		object["timestamp"] = int64(record.GetTimeUnixNano())
	case record.GetObservedTimeUnixNano() != 0:
		object["timestamp"] = int64(record.GetObservedTimeUnixNano())
	}
	if n := record.GetSeverityNumber(); n != 0 {
		object["severity"] = int64(n)
	} else if text := record.GetSeverityText(); text != "" {
		object["severity"] = text
	}
	if id := record.GetTraceId(); len(id) > 0 && !allZero(id) {
		object["trace_id"] = hex.EncodeToString(id)
	}
	if id := record.GetSpanId(); len(id) > 0 && !allZero(id) {
		object["span_id"] = hex.EncodeToString(id)
	}
	// OpenTelemetry semantic conventions carry the event name and request id as attributes.
	if name, ok := attributes["event.name"].(string); ok {
		object["event"] = name
	}
	for _, key := range []string{"request.id", "http.request.id", "x-request-id"} {
		if id, ok := attributes[key].(string); ok && id != "" {
			object["request_id"] = id
			break
		}
	}
	return object
}

func allZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

func keyValues(kvs []*commonpb.KeyValue) map[string]any {
	if len(kvs) == 0 {
		return nil
	}
	out := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		out[kv.GetKey()] = anyValue(kv.GetValue())
	}
	return out
}

func anyValue(v *commonpb.AnyValue) any {
	if v == nil {
		return nil
	}
	switch value := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return value.StringValue
	case *commonpb.AnyValue_BoolValue:
		return value.BoolValue
	case *commonpb.AnyValue_IntValue:
		return value.IntValue
	case *commonpb.AnyValue_DoubleValue:
		return value.DoubleValue
	case *commonpb.AnyValue_BytesValue:
		return base64.StdEncoding.EncodeToString(value.BytesValue)
	case *commonpb.AnyValue_ArrayValue:
		items := make([]any, 0, len(value.ArrayValue.GetValues()))
		for _, item := range value.ArrayValue.GetValues() {
			items = append(items, anyValue(item))
		}
		return items
	case *commonpb.AnyValue_KvlistValue:
		return keyValues(value.KvlistValue.GetValues())
	}
	return nil
}

// OTLPResponse encodes the ExportLogsServiceResponse for the given format, with a
// partial_success block when some records were rejected (per the OTLP spec a fully
// accepted request has an empty response).
func OTLPResponse(format OTLPFormat, rejected int, errs []EntryError) ([]byte, string, error) {
	response := &collogs.ExportLogsServiceResponse{}
	if rejected > 0 {
		messages := make([]string, 0, len(errs))
		for _, e := range errs {
			if len(messages) == 5 {
				break
			}
			messages = append(messages, fmt.Sprintf("record %d: %s", e.Index, e.Message))
		}
		response.PartialSuccess = &collogs.ExportLogsPartialSuccess{RejectedLogRecords: int64(rejected), ErrorMessage: strings.Join(messages, "; ")}
	}
	if format == OTLPProtobuf {
		b, err := proto.Marshal(response)
		return b, "application/x-protobuf", err
	}
	b, err := protojson.Marshal(response)
	return b, "application/json", err
}
