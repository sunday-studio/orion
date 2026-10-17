package logstore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrUnsupportedMedia is returned for a content type that is not JSON or NDJSON.
var ErrUnsupportedMedia = errors.New("unsupported content type")

// ErrInvalidBody is returned when the request is not parseable at all.
var ErrInvalidBody = errors.New("request body is not valid JSON")

// ErrTooManyEntries is returned when one request carries more entries than MaxEntriesPerRequest.
var ErrTooManyEntries = errors.New("too many entries in one request")

const (
	// MaxEntriesPerRequest keeps one request within what the queue can ever accept.
	MaxEntriesPerRequest  = 10_000
	MaxRequestBytes       = 1 << 20
	MaxNDJSONRequestBytes = 5 << 20
)

// Parsed is the result of reading one ingest request: valid entries plus per-entry errors.
type Parsed struct {
	Entries []Entry
	Errors  []EntryError
}

// ParseRequest reads a native ingest body. Content-Type decides the framing:
// application/json is one object or an array of objects; application/x-ndjson is one
// object per line. Bad entries are reported and skipped; only a body that cannot be
// framed at all is an error.
func ParseRequest(contentType string, body []byte, now time.Time) (Parsed, error) {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	var chunks []json.RawMessage
	var framingErrors []EntryError

	switch mediaType {
	case "application/x-ndjson", "application/ndjson", "application/jsonl":
		scanner := bufio.NewScanner(bytes.NewReader(body))
		scanner.Buffer(make([]byte, 0, 64<<10), MaxNDJSONRequestBytes)
		line := -1
		for scanner.Scan() {
			line++
			text := bytes.TrimSpace(scanner.Bytes())
			if len(text) == 0 {
				continue
			}
			if !json.Valid(text) {
				framingErrors = append(framingErrors, EntryError{Index: line, Code: "invalid_json", Message: "line is not valid JSON"})
				continue
			}
			chunks = append(chunks, append(json.RawMessage(nil), text...))
		}
		if err := scanner.Err(); err != nil {
			return Parsed{}, fmt.Errorf("%w: %v", ErrInvalidBody, err)
		}
		if len(chunks) > MaxEntriesPerRequest {
			return Parsed{}, ErrTooManyEntries
		}
		return normalizeChunks(chunks, framingErrors, now, true), nil
	case "application/json", "text/json":
		trimmed := bytes.TrimSpace(body)
		if len(trimmed) == 0 || !json.Valid(trimmed) {
			return Parsed{}, ErrInvalidBody
		}
		switch trimmed[0] {
		case '[':
			if err := json.Unmarshal(trimmed, &chunks); err != nil {
				return Parsed{}, fmt.Errorf("%w: %v", ErrInvalidBody, err)
			}
		case '{':
			chunks = []json.RawMessage{append(json.RawMessage(nil), trimmed...)}
		default:
			return Parsed{}, ErrInvalidBody
		}
		if len(chunks) > MaxEntriesPerRequest {
			return Parsed{}, ErrTooManyEntries
		}
		return normalizeChunks(chunks, nil, now, false), nil
	default:
		return Parsed{}, ErrUnsupportedMedia
	}
}

func normalizeChunks(chunks []json.RawMessage, framingErrors []EntryError, now time.Time, ndjson bool) Parsed {
	_ = ndjson
	result := Parsed{Errors: framingErrors}
	for i, chunk := range chunks {
		var object map[string]any
		decoder := json.NewDecoder(bytes.NewReader(chunk))
		decoder.UseNumber()
		if err := decoder.Decode(&object); err != nil || object == nil {
			result.Errors = append(result.Errors, EntryError{Index: i, Code: "invalid_json", Message: "entry must be a JSON object"})
			continue
		}
		numbersToFloat(object)
		entry, entryErr := Normalize(object, len(chunk), now)
		if entryErr != nil {
			entryErr.Index = i
			result.Errors = append(result.Errors, *entryErr)
			continue
		}
		result.Entries = append(result.Entries, entry)
	}
	return result
}

// numbersToFloat converts json.Number to float64 (or int64 when exact) so ids and
// large counters keep their precision in storage but compare naturally in code.
func numbersToFloat(v any) {
	switch value := v.(type) {
	case map[string]any:
		for k, child := range value {
			if n, ok := child.(json.Number); ok {
				if i, err := n.Int64(); err == nil {
					value[k] = i
				} else if f, err := n.Float64(); err == nil {
					value[k] = f
				}
				continue
			}
			numbersToFloat(child)
		}
	case []any:
		for i, child := range value {
			if n, ok := child.(json.Number); ok {
				if iv, err := n.Int64(); err == nil {
					value[i] = iv
				} else if f, err := n.Float64(); err == nil {
					value[i] = f
				}
				continue
			}
			numbersToFloat(child)
		}
	}
}
