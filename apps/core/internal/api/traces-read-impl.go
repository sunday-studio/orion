package api

import (
	"context"
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/logstore"
	"orion/core/internal/tracestore"
	"orion/core/internal/utils"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	traceQueryTimeout = 10 * time.Second
	// defaultFacetWindow bounds the facets of a request that names no start.
	defaultFacetWindow = 24 * time.Hour
	// maxTraceSummaryWindow bounds the range of a service summary.
	maxTraceSummaryWindow = 7 * 24 * time.Hour
)

var traceIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

// traceContext bounds a trace query by the request and the query timeout.
func traceContext(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), traceQueryTimeout)
}

// traceQueryFailed answers a failed trace query: 400 for a bad filter, 504 for a timeout.
func traceQueryFailed(c *gin.Context, err error, what string) {
	switch {
	case errors.Is(err, tracestore.ErrBadFilter):
		utils.BadRequest(c, err.Error())
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "interrupted"):
		c.JSON(http.StatusGatewayTimeout, utils.APIResponse{Success: false, Message: "The search timed out. Narrow the time range or add a filter.", Data: SearchTimeoutBody{Code: "search_timeout"}})
	default:
		utils.InternalError(c, what, err)
	}
}

// parseMinDuration reads a Go duration ("250ms", "1.5s") or a bare number of milliseconds.
func parseMinDuration(value string) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, true
	}
	if d, err := time.ParseDuration(value); err == nil && d >= 0 {
		return d, true
	}
	if ms, err := strconv.ParseFloat(value, 64); err == nil && ms >= 0 && ms < 1e9 {
		return time.Duration(ms * float64(time.Millisecond)), true
	}
	return 0, false
}

// serviceAppIDs returns the apps linked to a service, or notFound when the service does not exist.
func (s *Server) serviceAppIDs(serviceID string) (ids []string, found bool, err error) {
	var n int64
	if err := s.db.Model(&db.Service{}).Where("id = ?", serviceID).Count(&n).Error; err != nil || n == 0 {
		return nil, false, err
	}
	err = s.db.Model(&db.App{}).Where("service_id = ?", serviceID).Order("name ASC").Pluck("id", &ids).Error
	return ids, true, err
}

// traceQuery reads the shared filters of the list and facets endpoints. empty is true when the filters
// can match nothing (a service without apps), so the caller answers with no rows and runs no query.
func (s *Server) traceQuery(c *gin.Context, now time.Time) (q tracestore.Query, empty bool, ok bool) {
	from, okFrom := parseLogTime(strings.TrimSpace(c.Query("from")), now)
	to, okTo := parseLogTime(strings.TrimSpace(c.Query("to")), now)
	if !okFrom || !okTo {
		utils.BadRequest(c, "from and to must be RFC 3339 times or relative offsets like -1h")
		return q, false, false
	}
	minDuration, okDuration := parseMinDuration(c.Query("min_duration"))
	if !okDuration {
		utils.BadRequest(c, "min_duration must be a duration such as 250ms or 2s, or a number of milliseconds")
		return q, false, false
	}
	appIDs := splitList(c.Query("app_id"))
	if serviceID := strings.TrimSpace(c.Query("service_id")); serviceID != "" {
		linked, found, err := s.serviceAppIDs(serviceID)
		if err != nil {
			utils.InternalError(c, "Failed to resolve the service's apps", err)
			return q, false, false
		}
		if !found {
			utils.NotFound(c, "Service not found")
			return q, false, false
		}
		appIDs = intersectIDs(appIDs, linked)
		if len(appIDs) == 0 {
			empty = true
		}
	}
	q = tracestore.Query{
		AppIDs: appIDs, Service: strings.TrimSpace(c.Query("service")), Status: strings.TrimSpace(c.Query("status")),
		MinDuration: minDuration, Name: strings.TrimSpace(c.Query("name")), From: from, To: to, Attrs: c.QueryArray("attr"),
	}
	return q, empty, true
}

// intersectIDs keeps the linked apps that the app_id filter names; with no app_id filter all linked apps stay.
func intersectIDs(requested, linked []string) []string {
	if len(requested) == 0 {
		return linked
	}
	want := map[string]bool{}
	for _, id := range requested {
		want[id] = true
	}
	var out []string
	for _, id := range linked {
		if want[id] {
			out = append(out, id)
		}
	}
	return out
}

func (s *Server) listTracesImpl(c *gin.Context) {
	if !s.requireTraces(c) {
		return
	}
	q, empty, ok := s.traceQuery(c, time.Now().UTC())
	if !ok {
		return
	}
	if empty {
		utils.SuccessResponse(c, http.StatusOK, "Traces retrieved successfully", TraceListResponse{Traces: []TraceListItem{}})
		return
	}
	ctx, cancel := traceContext(c)
	defer cancel()
	q.Ctx, q.Limit, q.Cursor = ctx, utils.QueryInt(c, "limit", tracestore.DefaultPageSize), c.Query("cursor")
	traces, next, err := s.traces.ListTraces(q)
	if err != nil {
		traceQueryFailed(c, err, "Failed to search traces")
		return
	}
	ids := make([]string, 0, len(traces))
	seen := map[string]bool{}
	for _, t := range traces {
		if !seen[t.AppID] {
			seen[t.AppID] = true
			ids = append(ids, t.AppID)
		}
	}
	names := s.appNames(ids)
	out := make([]TraceListItem, 0, len(traces))
	for _, t := range traces {
		out = append(out, TraceListItem{
			TraceID: t.TraceID, AppID: t.AppID, AppName: names[t.AppID], ServiceName: t.ServiceName, RootName: t.RootName, StartTime: t.Start,
			DurationNs: t.DurationNs, SpanCount: t.SpanCount, ErrorCount: t.ErrorCount, HasRoot: t.HasRoot, Status: traceStatus(t.ErrorCount),
		})
	}
	utils.SuccessResponse(c, http.StatusOK, "Traces retrieved successfully", TraceListResponse{Traces: out, NextCursor: next})
}

func traceStatus(errorCount int64) TraceStatus {
	if errorCount > 0 {
		return TraceStatusError
	}
	return TraceStatusOK
}

func (s *Server) getTraceFacetsImpl(c *gin.Context) {
	if !s.requireTraces(c) {
		return
	}
	now := time.Now().UTC()
	q, empty, ok := s.traceQuery(c, now)
	if !ok {
		return
	}
	out := TraceFacetsResponse{Services: []TraceFacetCount{}, Names: []TraceFacetCount{}, Status: []TraceFacetCount{{Value: "ok"}, {Value: "error"}}}
	if empty {
		utils.SuccessResponse(c, http.StatusOK, "Trace facets retrieved successfully", out)
		return
	}
	if q.From.IsZero() {
		q.From = now.Add(-defaultFacetWindow)
	}
	ctx, cancel := traceContext(c)
	defer cancel()
	q.Ctx = ctx
	facets, err := s.traces.Facets(q)
	if err != nil {
		traceQueryFailed(c, err, "Failed to compute trace facets")
		return
	}
	out.Total, out.Status = facets.Total, toFacetCounts(facets.Status)
	out.Services, out.Names = toFacetCounts(facets.Services), toFacetCounts(facets.Names)
	utils.SuccessResponse(c, http.StatusOK, "Trace facets retrieved successfully", out)
}

func toFacetCounts(in []tracestore.Count) []TraceFacetCount {
	out := make([]TraceFacetCount, 0, len(in))
	for _, c := range in {
		out = append(out, TraceFacetCount{Value: c.Value, Count: c.Count})
	}
	return out
}

func (s *Server) getTraceImpl(c *gin.Context) {
	if !s.requireTraces(c) {
		return
	}
	id := strings.ToLower(strings.TrimSpace(c.Param("trace_id")))
	if !traceIDPattern.MatchString(id) {
		utils.BadRequest(c, "trace_id must be 32 hexadecimal characters")
		return
	}
	ctx, cancel := traceContext(c)
	defer cancel()
	found, err := s.traces.GetTrace(ctx, id)
	if err != nil {
		traceQueryFailed(c, err, "Failed to load the trace")
		return
	}
	if found.Total == 0 {
		utils.NotFound(c, "Trace not found")
		return
	}
	out := buildTraceDetail(id, found, s.appNamesFor(found.Spans))
	if s.logs != nil {
		if n, err := s.logs.store.Count(logstore.Query{TraceID: id, Ctx: ctx}); err == nil {
			out.LogCount = n
		}
	}
	utils.SuccessResponse(c, http.StatusOK, "Trace retrieved successfully", out)
}

func (s *Server) appNamesFor(spans []tracestore.Span) map[string]string {
	seen := map[string]bool{}
	var ids []string
	for _, sp := range spans {
		if !seen[sp.AppID] {
			seen[sp.AppID] = true
			ids = append(ids, sp.AppID)
		}
	}
	return s.appNames(ids)
}

// buildTraceDetail shapes the spans of a trace and derives its summary: the root span (the earliest span with
// no parent, else the earliest span) names it, and offsets count from the first span's start.
func buildTraceDetail(id string, found tracestore.TraceSpans, names map[string]string) TraceDetailResponse {
	out := TraceDetailResponse{TraceID: id, SpanCount: found.Total, Truncated: found.Total > int64(len(found.Spans)), Spans: make([]TraceSpan, 0, len(found.Spans))}
	if len(found.Spans) == 0 {
		return out
	}
	first := found.Spans[0].StartNano // spans arrive ordered by start
	var maxEnd int64
	head := 0 // the earliest root span, else the earliest span
	for i, sp := range found.Spans {
		if sp.ParentSpanID == "" {
			head = i
			break
		}
	}
	for _, sp := range found.Spans {
		maxEnd = max(maxEnd, sp.EndNano())
		if sp.StatusCode == tracestore.StatusError {
			out.ErrorCount++
		}
		events := make([]TraceSpanEvent, 0, len(sp.Events))
		for _, e := range sp.Events {
			events = append(events, TraceSpanEvent{Time: time.Unix(0, e.TimeUnixNano).UTC(), OffsetNs: e.TimeUnixNano - first, Name: e.Name, Attributes: e.Attributes})
		}
		links := make([]TraceSpanLink, 0, len(sp.Links))
		for _, l := range sp.Links {
			links = append(links, TraceSpanLink{TraceID: l.TraceID, SpanID: l.SpanID, Attributes: l.Attributes})
		}
		out.Spans = append(out.Spans, TraceSpan{
			SpanID: sp.SpanID, ParentSpanID: sp.ParentSpanID, AppID: sp.AppID, AppName: names[sp.AppID], Name: sp.Name, Kind: tracestore.KindName(sp.Kind),
			ServiceName: sp.ServiceName, StartTime: time.Unix(0, sp.StartNano).UTC(), StartOffsetNs: sp.StartNano - first, DurationNs: sp.DurationNano,
			Status: tracestore.StatusName(sp.StatusCode), StatusMessage: sp.StatusMessage, Attributes: sp.Attributes, Resource: sp.Resource, Events: events, Links: links,
		})
	}
	root := found.Spans[head]
	out.RootName, out.ServiceName, out.StartTime = root.Name, root.ServiceName, time.Unix(0, root.StartNano).UTC()
	out.HasRoot = root.ParentSpanID == ""
	out.DurationNs = root.DurationNano
	if !out.HasRoot {
		out.DurationNs = maxEnd - first
	}
	out.Status = traceStatus(out.ErrorCount)
	return out
}

func (s *Server) getServiceTraceSummaryImpl(c *gin.Context) {
	if !s.requireTraces(c) {
		return
	}
	now := time.Now().UTC()
	from, okFrom := parseLogTime(strings.TrimSpace(c.Query("from")), now)
	to, okTo := parseLogTime(strings.TrimSpace(c.Query("to")), now)
	if !okFrom || !okTo {
		utils.BadRequest(c, "from and to must be RFC 3339 times or relative offsets such as -1h")
		return
	}
	if to.IsZero() {
		to = now
	}
	if from.IsZero() {
		from = to.Add(-time.Hour)
	}
	switch {
	case !from.Before(to):
		utils.BadRequest(c, "from must be before to")
		return
	case to.Sub(from) > maxTraceSummaryWindow:
		utils.BadRequest(c, "The range is limited to 7 days")
		return
	}
	appIDs, found, err := s.serviceAppIDs(c.Param("id"))
	if err != nil {
		utils.InternalError(c, "Failed to resolve the service's apps", err)
		return
	}
	if !found {
		utils.NotFound(c, "Service not found")
		return
	}
	ctx, cancel := traceContext(c)
	defer cancel()
	summary, err := s.traces.Summarize(ctx, appIDs, from, to)
	if err != nil {
		traceQueryFailed(c, err, "Failed to summarise traces")
		return
	}
	out := ServiceTraceSummaryResponse{
		ServiceID: c.Param("id"), From: from, To: to, AppIDs: appIDs, TraceCount: summary.TraceCount, ErrorCount: summary.ErrorCount,
		RootedCount: summary.RootedCount, Sampled: summary.Sampled,
	}
	if out.AppIDs == nil {
		out.AppIDs = []string{}
	}
	if summary.TraceCount > 0 {
		out.ErrorRate = float64(summary.ErrorCount) / float64(summary.TraceCount)
	}
	if summary.RootedCount > 0 {
		out.P50Ns, out.P95Ns, out.P99Ns = &summary.P50, &summary.P95, &summary.P99
	}
	utils.SuccessResponse(c, http.StatusOK, "Service trace summary retrieved successfully", out)
}
