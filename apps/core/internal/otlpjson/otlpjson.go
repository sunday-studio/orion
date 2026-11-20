// Package otlpjson adapts OTLP/JSON bodies to what protojson can decode.
//
// The OTLP/JSON encoding makes one exception to the protobuf JSON mapping: trace and span
// ids are hex strings, not base64. Every OpenTelemetry SDK and the Collector send hex,
// while protojson only reads base64 for bytes fields. NormalizeIDs rewrites the hex ids
// into base64 so one decoder handles both forms.
package otlpjson

import "encoding/base64"

const (
	traceIDHexLen = 32
	spanIDHexLen  = 16
)

// hexLength says how long the hex form of the id named key is, or zero for other keys.
func hexLength(key []byte) int {
	switch string(key) {
	case "traceId", "trace_id":
		return traceIDHexLen
	case "spanId", "span_id", "parentSpanId", "parent_span_id":
		return spanIDHexLen
	}
	return 0
}

// NormalizeIDs returns body with every traceId, spanId and parentSpanId (or snake_case)
// object member whose value is a string of exactly 32 (trace) or 16 (span) hex characters
// rewritten to the base64 form protojson reads, at any depth. Anything else (an empty
// string, a base64 value, a malformed id) is left untouched for protojson to accept or
// reject as before.
//
// It makes one linear pass over the bytes without building a tree, so its cost is bounded
// by the body size that the callers already cap, and it returns the input slice itself
// when nothing needs rewriting. The input is never modified.
func NormalizeIDs(body []byte) []byte {
	var out []byte // allocated at the first rewrite
	copied := 0    // body[:copied] is already in out
	n := len(body)
	for i := 0; i < n; {
		if body[i] != '"' {
			i++
			continue
		}
		keyStart := i + 1
		keyEnd, ok := stringEnd(body, keyStart)
		if !ok {
			break
		}
		i = keyEnd + 1
		// A string followed by a colon is an object key; every other string is a value.
		j := skipSpace(body, i)
		if j >= n || body[j] != ':' {
			continue
		}
		j = skipSpace(body, j+1)
		hexLen := hexLength(body[keyStart:keyEnd])
		if hexLen == 0 || j >= n || body[j] != '"' {
			i = j
			continue
		}
		valStart := j + 1
		valEnd, ok := stringEnd(body, valStart)
		if !ok {
			break
		}
		i = valEnd + 1
		if valEnd-valStart != hexLen {
			continue
		}
		raw, ok := decodeHex(body[valStart:valEnd])
		if !ok {
			continue
		}
		if out == nil {
			out = make([]byte, 0, n)
		}
		out = append(out, body[copied:valStart]...)
		out = base64.StdEncoding.AppendEncode(out, raw)
		copied = valEnd
	}
	if out == nil {
		return body
	}
	return append(out, body[copied:]...)
}

// stringEnd returns the index of the closing quote of the string whose content starts at start.
func stringEnd(b []byte, start int) (int, bool) {
	for i := start; i < len(b); i++ {
		switch b[i] {
		case '\\':
			i++
		case '"':
			return i, true
		}
	}
	return 0, false
}

func skipSpace(b []byte, i int) int {
	for i < len(b) {
		switch b[i] {
		case ' ', '\t', '\r', '\n':
			i++
		default:
			return i
		}
	}
	return i
}

func hexValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// decodeHex decodes an even-length hex string; ok is false when a character is not hex.
func decodeHex(src []byte) ([]byte, bool) {
	dst := make([]byte, len(src)/2)
	for i := range dst {
		hi, lo := hexValue(src[2*i]), hexValue(src[2*i+1])
		if hi < 0 || lo < 0 {
			return nil, false
		}
		dst[i] = byte(hi<<4 | lo)
	}
	return dst, true
}
