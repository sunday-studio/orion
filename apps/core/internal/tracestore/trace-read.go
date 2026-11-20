package tracestore

import (
	"context"
	"encoding/json"
	"time"
)

type spanRow struct {
	AppID         string
	TraceID       string
	SpanID        string
	ParentSpanID  string
	Name          string
	Kind          int
	StartNs       int64
	DurationNs    int64
	StatusCode    int
	StatusMessage string
	ServiceName   string
	Attributes    string
	Resource      string
	Events        string
	Links         string
	IngestedAt    int64
	Size          int
}

func (r spanRow) span() Span {
	sp := Span{
		AppID: r.AppID, TraceID: r.TraceID, SpanID: r.SpanID, ParentSpanID: r.ParentSpanID, Name: r.Name, Kind: r.Kind,
		StartNano: r.StartNs, DurationNano: r.DurationNs, StatusCode: r.StatusCode, StatusMessage: r.StatusMessage, ServiceName: r.ServiceName,
		IngestedAt: time.Unix(0, r.IngestedAt).UTC(), Size: r.Size,
	}
	sp.Attributes = decodeObject(r.Attributes)
	sp.Resource = decodeObject(r.Resource)
	if r.Events != "" {
		_ = json.Unmarshal([]byte(r.Events), &sp.Events)
	}
	if r.Links != "" {
		_ = json.Unmarshal([]byte(r.Links), &sp.Links)
	}
	return sp
}

func decodeObject(s string) map[string]any {
	if s == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

// TraceSpans is every stored span of one trace id, across apps.
type TraceSpans struct {
	Spans []Span
	// Total is how many spans the trace has; Spans holds at most MaxSpansPerTraceRead of them.
	Total int64
}

// GetTrace returns the spans of a trace id ordered by start time, or zero Total when none is stored.
// Apps that instrument both sides of a call share the trace id, so the spans of all apps are returned.
func (s *Store) GetTrace(ctx context.Context, traceID string) (TraceSpans, error) {
	var out TraceSpans
	db := s.dbq(ctx)
	if err := db.Raw("SELECT COUNT(*) FROM spans WHERE trace_id = ?", traceID).Scan(&out.Total).Error; err != nil || out.Total == 0 {
		return out, err
	}
	var rows []spanRow
	if err := db.Raw("SELECT * FROM spans WHERE trace_id = ? ORDER BY start_ns ASC, span_id ASC LIMIT ?", traceID, MaxSpansPerTraceRead).Scan(&rows).Error; err != nil {
		return out, err
	}
	out.Spans = make([]Span, len(rows))
	for i, r := range rows {
		out.Spans[i] = r.span()
	}
	return out, nil
}

// CountForApp returns stored spans and bytes for an app (retention, diagnostics, tests).
func (s *Store) CountForApp(appID string) (count int64, bytes int64, err error) {
	row := struct {
		N int64
		B int64
	}{}
	err = s.db.Raw("SELECT COUNT(*) AS n, COALESCE(SUM(size),0) AS b FROM spans WHERE app_id = ?", appID).Scan(&row).Error
	return row.N, row.B, err
}
