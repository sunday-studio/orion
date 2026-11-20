package tracestore

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

var attributeKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_.\-@/]{1,128}$`)

// Query describes a trace search. Zero values mean "no filter".
type Query struct {
	AppIDs []string
	// Service matches the service of the trace's root span (its entry point).
	Service string
	// Status is "ok" (no span failed), "error" (at least one span failed) or "".
	Status      string
	MinDuration time.Duration
	// Name matches a substring of the root span's name.
	Name     string
	From, To time.Time
	// Attrs are key:value pairs; a trace matches when one of its spans (or its resource) carries the value.
	Attrs  []string
	Limit  int
	Cursor string
	// Ctx bounds the database work of this query (nil = unbounded).
	Ctx context.Context
}

func (s *Store) dbq(ctx context.Context) *gorm.DB {
	if ctx != nil {
		return s.db.WithContext(ctx)
	}
	return s.db
}

type traceRow struct {
	AppID       string
	TraceID     string
	RootName    string
	ServiceName string
	StartNs     int64
	DurationNs  int64
	SpanCount   int64
	ErrorCount  int64
	HasRoot     bool
}

func (r traceRow) trace() Trace {
	return Trace{AppID: r.AppID, TraceID: r.TraceID, RootName: r.RootName, ServiceName: r.ServiceName, Start: time.Unix(0, r.StartNs).UTC(),
		DurationNs: r.DurationNs, SpanCount: r.SpanCount, ErrorCount: r.ErrorCount, HasRoot: r.HasRoot}
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	return strings.ReplaceAll(s, "_", `\_`)
}

// where builds the filter over the traces table (aliased t). The cursor is applied separately
// because facets and summaries ignore it.
func (q Query) where() (string, []any, error) {
	var clauses []string
	var args []any
	if len(q.AppIDs) > 0 {
		clauses = append(clauses, "t.app_id IN ?")
		args = append(args, q.AppIDs)
	}
	if q.Service != "" {
		clauses = append(clauses, "t.service_name = ?")
		args = append(args, q.Service)
	}
	switch q.Status {
	case "":
	case "error":
		clauses = append(clauses, "t.error_count > 0")
	case "ok":
		clauses = append(clauses, "t.error_count = 0")
	default:
		return "", nil, fmt.Errorf("%w: status must be ok or error", ErrBadFilter)
	}
	if q.MinDuration > 0 {
		clauses = append(clauses, "t.duration_ns >= ?")
		args = append(args, q.MinDuration.Nanoseconds())
	}
	if q.Name != "" {
		clauses = append(clauses, `t.root_name LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(q.Name)+"%")
	}
	if !q.From.IsZero() {
		clauses = append(clauses, "t.start_ns >= ?")
		args = append(args, q.From.UnixNano())
	}
	if !q.To.IsZero() {
		clauses = append(clauses, "t.start_ns <= ?")
		args = append(args, q.To.UnixNano())
	}
	for _, attr := range q.Attrs {
		key, value, found := strings.Cut(attr, ":")
		if !found || !validAttributeKey(key) {
			return "", nil, fmt.Errorf("%w: attr must be key:value with a plain key, got %q", ErrBadFilter, attr)
		}
		// The key is validated above and quoted into the JSON path; the value is bound.
		clauses = append(clauses, "EXISTS (SELECT 1 FROM spans s WHERE s.trace_id = t.trace_id AND s.app_id = t.app_id AND ("+attrExpr("s.attributes", key)+" = ? OR "+attrExpr("s.resource", key)+" = ?))")
		args = append(args, value, value)
	}
	if len(clauses) == 0 {
		return "", nil, nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}

// ListTraces returns one page, newest first, plus the cursor for the next page ("" at the end).
func (s *Store) ListTraces(q Query) ([]Trace, string, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	limit = min(limit, MaxPageSize)
	where, args, err := q.where()
	if err != nil {
		return nil, "", err
	}
	if q.Cursor != "" {
		ts, traceID, appID, err := decodeCursor(q.Cursor)
		if err != nil {
			return nil, "", err
		}
		clause := "(t.start_ns < ? OR (t.start_ns = ? AND (t.trace_id < ? OR (t.trace_id = ? AND t.app_id < ?))))"
		if where == "" {
			where = " WHERE " + clause
		} else {
			where += " AND " + clause
		}
		args = append(args, ts, ts, traceID, traceID, appID)
	}
	args = append(args, limit+1)
	var rows []traceRow
	if err := s.dbq(q.Ctx).Raw("SELECT t.app_id, t.trace_id, t.root_name, t.service_name, t.start_ns, t.duration_ns, t.span_count, t.error_count, t.has_root FROM traces t"+where+
		" ORDER BY t.start_ns DESC, t.trace_id DESC, t.app_id DESC LIMIT ?", args...).Scan(&rows).Error; err != nil {
		return nil, "", err
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next = encodeCursor(last.StartNs, last.TraceID, last.AppID)
	}
	out := make([]Trace, len(rows))
	for i, r := range rows {
		out[i] = r.trace()
	}
	return out, next, nil
}

func encodeCursor(ts int64, traceID, appID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(ts, 10) + "|" + traceID + "|" + appID))
}

func decodeCursor(cursor string) (int64, string, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, "", "", fmt.Errorf("%w: bad cursor", ErrBadFilter)
	}
	parts := strings.SplitN(string(raw), "|", 3)
	if len(parts) != 3 {
		return 0, "", "", fmt.Errorf("%w: bad cursor", ErrBadFilter)
	}
	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", "", fmt.Errorf("%w: bad cursor", ErrBadFilter)
	}
	return ts, parts[1], parts[2], nil
}

// attrExpr reads one attribute of a JSON column as text. Rows without attributes store an empty
// string, which json_extract rejects, so those rows are guarded. A dotted key matches a literal key
// ("http.status") or a nested object (http: {status}).
func attrExpr(column, key string) string {
	literal := fmt.Sprintf(`json_extract(%s, '$."%s"')`, column, key)
	if !strings.Contains(key, ".") {
		return fmt.Sprintf(`CASE WHEN %s <> '' THEN CAST(%s AS TEXT) END`, column, literal)
	}
	nested := fmt.Sprintf(`json_extract(%s, '$.%s')`, column, key)
	return fmt.Sprintf(`CASE WHEN %s <> '' THEN CAST(COALESCE(%s, %s) AS TEXT) END`, column, literal, nested)
}

// validAttributeKey accepts plain attribute keys; empty dot segments ("a..b", ".a") are not valid JSON paths.
func validAttributeKey(key string) bool {
	if !attributeKeyPattern.MatchString(key) {
		return false
	}
	for _, part := range strings.Split(key, ".") {
		if part == "" {
			return false
		}
	}
	return true
}
