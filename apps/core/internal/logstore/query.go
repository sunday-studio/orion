package logstore

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

// ErrBadFilter marks a filter the caller got wrong (maps to HTTP 400).
var ErrBadFilter = errors.New("invalid log filter")

var attributeKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_.\-@/]{1,128}$`)

// Query describes a log search. Zero values mean "no filter".
type Query struct {
	AppIDs    []string
	Severity  string // "error,fatal" or ">=warn"
	Event     string
	Text      string
	From, To  time.Time
	TraceID   string
	RequestID string
	Attrs     []string // key:value
	Limit     int
	Cursor    string
}

type logRow struct {
	ID             string
	AppID          string
	Ts             int64
	IngestedAt     int64
	Severity       string
	SeverityNumber int
	Event          string
	Body           string
	BodyJSON       string
	Attributes     string
	Resource       string
	TraceID        string
	SpanID         string
	RequestID      string
}

func (r logRow) entry() Entry {
	e := Entry{
		ID: r.ID, AppID: r.AppID, Timestamp: time.Unix(0, r.Ts).UTC(), IngestedAt: time.Unix(0, r.IngestedAt).UTC(),
		Severity: r.Severity, SeverityNumber: r.SeverityNumber, Event: r.Event, Body: r.Body, BodyJSON: r.BodyJSON,
		Attributes: decodeObject(r.Attributes), Resource: decodeObject(r.Resource), TraceID: r.TraceID, SpanID: r.SpanID, RequestID: r.RequestID,
	}
	return e
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	return strings.ReplaceAll(s, "_", `\_`)
}

func (q Query) where() (string, []any, error) {
	var clauses []string
	var args []any
	if len(q.AppIDs) > 0 {
		clauses = append(clauses, "app_id IN ?")
		args = append(args, q.AppIDs)
	}
	if q.Severity != "" {
		if min, max, ok := SeverityRange(q.Severity); ok {
			clauses = append(clauses, "severity_number BETWEEN ? AND ?")
			args = append(args, min, max)
		} else {
			var names []string
			for _, name := range strings.Split(q.Severity, ",") {
				if name = strings.TrimSpace(name); name != "" {
					canonical, _ := NormalizeSeverity(name)
					names = append(names, canonical)
				}
			}
			if len(names) > 0 {
				clauses = append(clauses, "severity IN ?")
				args = append(args, names)
			}
		}
	}
	if q.Event != "" {
		clauses = append(clauses, "event = ?")
		args = append(args, q.Event)
	}
	if q.Text != "" {
		like := "%" + escapeLike(q.Text) + "%"
		clauses = append(clauses, `(body LIKE ? ESCAPE '\' OR event LIKE ? ESCAPE '\' OR attributes LIKE ? ESCAPE '\')`)
		args = append(args, like, like, like)
	}
	if !q.From.IsZero() {
		clauses = append(clauses, "ts >= ?")
		args = append(args, q.From.UnixNano())
	}
	if !q.To.IsZero() {
		clauses = append(clauses, "ts <= ?")
		args = append(args, q.To.UnixNano())
	}
	if q.TraceID != "" {
		clauses = append(clauses, "trace_id = ?")
		args = append(args, q.TraceID)
	}
	if q.RequestID != "" {
		clauses = append(clauses, "request_id = ?")
		args = append(args, q.RequestID)
	}
	for _, attr := range q.Attrs {
		key, value, found := strings.Cut(attr, ":")
		if !found || !attributeKeyPattern.MatchString(key) {
			return "", nil, fmt.Errorf("%w: attr must be key:value with a plain key, got %q", ErrBadFilter, attr)
		}
		// The key is validated above and quoted into the JSON path; the value is bound.
		clauses = append(clauses, fmt.Sprintf(`CAST(json_extract(attributes, '$."%s"') AS TEXT) = ?`, key))
		args = append(args, value)
	}
	if q.Cursor != "" {
		ts, id, err := decodeCursor(q.Cursor)
		if err != nil {
			return "", nil, err
		}
		clauses = append(clauses, "(ts < ? OR (ts = ? AND id < ?))")
		args = append(args, ts, ts, id)
	}
	if len(clauses) == 0 {
		return "", nil, nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}

// EncodeCursor and DecodeCursor expose the (time, id) paging cursor for other sources.
func EncodeCursor(ts int64, id string) string { return encodeCursor(ts, id) }

func DecodeCursor(cursor string) (int64, string, error) { return decodeCursor(cursor) }

func encodeCursor(ts int64, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(ts, 10) + "|" + id))
}

func decodeCursor(cursor string) (int64, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, "", fmt.Errorf("%w: bad cursor", ErrBadFilter)
	}
	tsText, id, found := strings.Cut(string(raw), "|")
	ts, parseErr := strconv.ParseInt(tsText, 10, 64)
	if !found || parseErr != nil {
		return 0, "", fmt.Errorf("%w: bad cursor", ErrBadFilter)
	}
	return ts, id, nil
}

// List returns one page, newest first, plus the cursor for the next page ("" at the end).
func (s *Store) List(q Query) ([]Entry, string, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	limit = min(limit, MaxPageSize)
	where, args, err := q.where()
	if err != nil {
		return nil, "", err
	}
	var rows []logRow
	args = append(args, limit+1)
	if err := s.db.Raw("SELECT * FROM log_entries"+where+" ORDER BY ts DESC, id DESC LIMIT ?", args...).Scan(&rows).Error; err != nil {
		return nil, "", err
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next = encodeCursor(last.Ts, last.ID)
	}
	entries := make([]Entry, len(rows))
	for i, r := range rows {
		entries[i] = r.entry()
	}
	return entries, next, nil
}

// Get returns one entry by id, or nil.
func (s *Store) Get(id string) (*Entry, error) {
	var rows []logRow
	if err := s.db.Raw("SELECT * FROM log_entries WHERE id = ? LIMIT 1", id).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	e := rows[0].entry()
	return &e, nil
}

// CountForApp returns stored entries and bytes for an app (diagnostics, tests).
func (s *Store) CountForApp(appID string) (count int64, bytes int64, err error) {
	row := struct {
		N int64
		B int64
	}{}
	err = s.db.Raw("SELECT COUNT(*) AS n, COALESCE(SUM(size),0) AS b FROM log_entries WHERE app_id = ?", appID).Scan(&row).Error
	return row.N, row.B, err
}

// Count returns how many entries match the query's filters (paging is ignored).
func (s *Store) Count(q Query) (int64, error) {
	q.Cursor = ""
	where, args, err := q.where()
	if err != nil {
		return 0, err
	}
	var n int64
	err = s.db.Raw("SELECT COUNT(*) FROM log_entries"+where, args...).Scan(&n).Error
	return n, err
}
