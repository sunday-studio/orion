package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"orion/core/internal/logstore"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const statusLogsDefaultWindow = 24 * time.Hour

var statusLevels = map[string]string{"info": "info", "warn": "warn", "warning": "warn", "error": "error", "err": "error"}

// StatusLogListResponse is one page of status logs.
type StatusLogListResponse struct {
	Entries []StatusLogEntry `json:"entries"`
	Count   int              `json:"count"`
	// Total is how many events match the filter in the window.
	Total int64 `json:"total"`
	// NextCursor pages to older events, PrevCursor to newer ones; empty at the ends.
	NextCursor string `json:"next_cursor,omitempty"`
	PrevCursor string `json:"prev_cursor,omitempty"`
}

type StatusLogFacet struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
	Count int64  `json:"count"`
}

// StatusLogFacetsResponse feeds the filter rail of the Status logs tab.
type StatusLogFacetsResponse struct {
	Total   int64            `json:"total"`
	Levels  map[string]int64 `json:"levels"`
	Types   map[string]int64 `json:"types"`
	Servers []StatusLogFacet `json:"servers"`
	// ServersDistinct is how many different servers appear, for "Show 3 more".
	ServersDistinct  int              `json:"servers_distinct"`
	Monitors         []StatusLogFacet `json:"monitors"`
	MonitorsDistinct int              `json:"monitors_distinct"`
}

// parseStatusFilter reads the shared filter parameters; it applies the 24 hour default window.
func parseStatusFilter(values url.Values, now time.Time) (statusFilter, error) {
	f := statusFilter{ServerID: values.Get("server_id"), MonitorID: values.Get("monitor_id"), IncidentID: values.Get("incident_id"), Text: strings.TrimSpace(values.Get("q"))}
	for _, raw := range splitList(values.Get("level")) {
		level, ok := statusLevels[strings.ToLower(raw)]
		if !ok {
			return f, fmt.Errorf("level must be info, warn or error")
		}
		f.Levels = append(f.Levels, level)
	}
	for _, raw := range splitList(values.Get("type")) {
		raw = strings.ToLower(raw)
		if raw != "monitor" && raw != "alert" && raw != "server" && raw != "incident" && raw != "system" {
			return f, fmt.Errorf("type must be monitor, alert, server, incident or system")
		}
		f.Types = append(f.Types, raw)
	}
	var ok bool
	if f.From, ok = parseLogTime(values.Get("from"), now); !ok {
		return f, fmt.Errorf("from and to must be RFC 3339 times or relative offsets like -1h")
	}
	if f.To, ok = parseLogTime(values.Get("to"), now); !ok {
		return f, fmt.Errorf("from and to must be RFC 3339 times or relative offsets like -1h")
	}
	if f.From.IsZero() {
		f.From = now.Add(-statusLogsDefaultWindow)
	}
	return f, nil
}

func (s *Server) statusFilterFromRequest(c *gin.Context) (statusFilter, bool) {
	f, err := parseStatusFilter(c.Request.URL.Query(), time.Now().UTC())
	if err != nil {
		utils.BadRequest(c, err.Error())
		return f, false
	}
	return f, true
}

func (s *Server) listStatusLogsImpl(c *gin.Context) {
	f, ok := s.statusFilterFromRequest(c)
	if !ok {
		return
	}
	limit := utils.QueryInt(c, "limit", 100)
	if limit < 1 {
		limit = 100
	}
	cursorMs, cursorID, hasCursor := int64(0), "", false
	if raw := c.Query("cursor"); raw != "" {
		var err error
		if cursorMs, cursorID, err = logstore.DecodeCursor(raw); err != nil {
			utils.BadRequest(c, "Invalid cursor")
			return
		}
		hasCursor = true
	}
	newer := c.Query("direction") == "newer"
	rows, err := statusPage(s.db, f, limit+1, cursorMs, cursorID, hasCursor, newer)
	if err != nil {
		s.logger.Error("Failed to list status logs", "error", err)
		utils.InternalError(c, "Failed to list status logs", err)
		return
	}
	more := len(rows) > limit
	if more {
		if newer {
			rows = rows[1:] // the extra row is the newest one
		} else {
			rows = rows[:limit]
		}
	}
	out := StatusLogListResponse{Entries: make([]StatusLogEntry, 0, len(rows))}
	for _, r := range rows {
		out.Entries = append(out.Entries, statusEntry(r))
	}
	out.Count = len(out.Entries)
	if len(rows) > 0 {
		first, last := rows[0], rows[len(rows)-1]
		switch {
		case newer:
			out.NextCursor = logstore.EncodeCursor(last.AtMs, last.ID)
			if more {
				out.PrevCursor = logstore.EncodeCursor(first.AtMs, first.ID)
			}
		default:
			if more {
				out.NextCursor = logstore.EncodeCursor(last.AtMs, last.ID)
			}
			if hasCursor {
				out.PrevCursor = logstore.EncodeCursor(first.AtMs, first.ID)
			}
		}
	}
	if out.Total, err = statusCount(s.db, f); err != nil {
		utils.InternalError(c, "Failed to count status logs", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status logs retrieved successfully", out)
}

func (s *Server) getStatusLogFacetsImpl(c *gin.Context) {
	f, ok := s.statusFilterFromRequest(c)
	if !ok {
		return
	}
	top := utils.QueryInt(c, "top", 3)
	if top < 1 || top > 50 {
		top = 3
	}
	out := StatusLogFacetsResponse{Levels: map[string]int64{}, Types: map[string]int64{}, Servers: []StatusLogFacet{}, Monitors: []StatusLogFacet{}}
	levels, _, err := statusGroups(s.db, f, "level", "", 0)
	if err != nil {
		utils.InternalError(c, "Failed to compute facets", err)
		return
	}
	for _, g := range levels {
		out.Levels[g.Key] = g.N
		out.Total += g.N
	}
	types, _, err := statusGroups(s.db, f, "type", "", 0)
	if err != nil {
		utils.InternalError(c, "Failed to compute facets", err)
		return
	}
	for _, g := range types {
		out.Types[g.Key] = g.N
	}
	servers, nServers, err := statusGroups(s.db, f, "server_id", "server_name", top)
	if err != nil {
		utils.InternalError(c, "Failed to compute facets", err)
		return
	}
	for _, g := range servers {
		out.Servers = append(out.Servers, StatusLogFacet{Value: g.Key, Label: g.Label, Count: g.N})
	}
	out.ServersDistinct = nServers
	monitors, nMonitors, err := statusGroups(s.db, f, "monitor_id", "monitor_name", top)
	if err != nil {
		utils.InternalError(c, "Failed to compute facets", err)
		return
	}
	for _, g := range monitors {
		out.Monitors = append(out.Monitors, StatusLogFacet{Value: g.Key, Label: g.Label, Count: g.N})
	}
	out.MonitorsDistinct = nMonitors
	utils.SuccessResponse(c, http.StatusOK, "Status log facets retrieved successfully", out)
}

var histogramSteps = []time.Duration{time.Second, 5 * time.Second, 15 * time.Second, 30 * time.Second, time.Minute, 5 * time.Minute, 15 * time.Minute, 30 * time.Minute, time.Hour, 3 * time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour}

func (s *Server) getStatusLogHistogramImpl(c *gin.Context) {
	now := time.Now().UTC()
	f, ok := s.statusFilterFromRequest(c)
	if !ok {
		return
	}
	to := f.To
	if to.IsZero() {
		to = now
	}
	if !to.After(f.From) {
		utils.BadRequest(c, "to must be after from")
		return
	}
	buckets := max(min(utils.QueryInt(c, "buckets", 60), 500), 1)
	width := histogramSteps[len(histogramSteps)-1]
	for _, step := range histogramSteps {
		if to.Sub(f.From)/step <= time.Duration(buckets) {
			width = step
			break
		}
	}
	rows, err := statusHistogram(s.db, f, width.Milliseconds())
	if err != nil {
		utils.InternalError(c, "Failed to compute the histogram", err)
		return
	}
	byStart := map[int64]*LogHistogramBucket{}
	for _, r := range rows {
		b := byStart[r.Bucket]
		if b == nil {
			b = &LogHistogramBucket{Start: time.UnixMilli(r.Bucket).UTC(), Severity: map[string]int{}}
			byStart[r.Bucket] = b
		}
		b.Severity[r.Level] += int(r.N)
		b.Total += r.N
	}
	out := []LogHistogramBucket{}
	w := width.Milliseconds()
	for start := (f.From.UnixMilli() / w) * w; start <= to.UnixMilli(); start += w {
		if b := byStart[start]; b != nil {
			out = append(out, *b)
		} else {
			out = append(out, LogHistogramBucket{Start: time.UnixMilli(start).UTC(), Severity: map[string]int{}})
		}
	}
	utils.SuccessResponse(c, http.StatusOK, "Status log histogram retrieved successfully", LogHistogramResponse{BucketSeconds: int(width.Seconds()), Buckets: out})
}

func severityNumber(level string) int {
	switch level {
	case "error":
		return 17
	case "warn":
		return 13
	default:
		return 9
	}
}

func (s *Server) getStatusLogPatternsImpl(c *gin.Context) {
	f, ok := s.statusFilterFromRequest(c)
	if !ok {
		return
	}
	to := f.To
	if to.IsZero() {
		to = time.Now().UTC()
	}
	read := func(ff statusFilter) ([]logstore.PatternInput, error) {
		rows, err := statusPage(s.db, ff, logstore.PatternsScanLimit, 0, "", false, false)
		if err != nil {
			return nil, err
		}
		out := make([]logstore.PatternInput, 0, len(rows))
		for _, r := range rows {
			e := statusEntry(r)
			out = append(out, logstore.PatternInput{ID: e.ID, Text: joinNonEmpty(": ", e.Title, e.Detail), Severity: e.Level, SevNum: severityNumber(e.Level), At: e.At})
		}
		return out, nil
	}
	f.To = to
	current, err := read(f)
	if err != nil {
		utils.InternalError(c, "Failed to compute patterns", err)
		return
	}
	prev := f
	span := to.Sub(f.From)
	prev.From, prev.To = f.From.Add(-span), f.From
	previous, err := read(prev)
	if err != nil {
		utils.InternalError(c, "Failed to compute patterns", err)
		return
	}
	clustered := logstore.Cluster(current, previous, f.From, to, 12, utils.QueryInt(c, "limit", 50))
	out := make([]LogPattern, 0, len(clustered))
	for _, p := range clustered {
		out = append(out, LogPattern{Pattern: p.Template, Severity: LogSeverity(p.Severity), Count: p.Count, Trend: p.Trend, FirstSeen: p.FirstSeen, LastSeen: p.LastSeen, IsNew: p.New, SampleID: p.SampleID})
	}
	utils.SuccessResponse(c, http.StatusOK, "Status log patterns retrieved successfully", LogPatternsResponse{Patterns: out, From: f.From, To: to})
}

// streamStatusLogsImpl follows new events live: it polls the sources every two seconds.
func (s *Server) streamStatusLogsImpl(c *gin.Context) {
	f, ok := s.statusFilterFromRequest(c)
	if !ok {
		return
	}
	f.From, f.To = time.Time{}, time.Time{}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		utils.InternalError(c, "Streaming is not supported by this connection", nil)
		return
	}
	controller := http.NewResponseController(c.Writer)
	lastMs, lastID := time.Now().UTC().UnixMilli(), "\xff"
	if resume := c.GetHeader("Last-Event-ID"); resume != "" {
		if ms, id, err := logstore.DecodeCursor(resume); err == nil {
			lastMs, lastID = ms, id
		}
	}
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	extend := func() { _ = controller.SetWriteDeadline(time.Now().Add(time.Minute)) }
	extend()
	fmt.Fprint(c.Writer, ": connected\n\n")
	flusher.Flush()
	poll := time.NewTicker(2 * time.Second)
	heartbeat := time.NewTicker(15 * time.Second)
	defer poll.Stop()
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeat.C:
			extend()
			if _, err := fmt.Fprint(c.Writer, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-poll.C:
			rows, err := statusPage(s.db, f, 200, lastMs, lastID, true, true)
			if err != nil {
				continue
			}
			extend()
			for _, r := range rows {
				payload, _ := json.Marshal(statusEntry(r))
				if _, err := fmt.Fprintf(c.Writer, "id: %s\ndata: %s\n\n", logstore.EncodeCursor(r.AtMs, r.ID), payload); err != nil {
					return
				}
				lastMs, lastID = r.AtMs, r.ID
			}
			flusher.Flush()
		}
	}
}

// statusExportSource streams status logs for the export job.
func (s *Server) statusExportSource() exportSource {
	return exportSource{
		header: []string{"timestamp", "level", "type", "server", "monitor", "title", "detail"},
		run: func(ctx context.Context, values url.Values, limit int, emit func([]string, map[string]any) error) error {
			f, err := parseStatusFilter(values, time.Now().UTC())
			if err != nil {
				return err
			}
			cursorMs, cursorID, has := int64(0), "", false
			sent := 0
			for sent < limit {
				rows, err := statusPage(s.db.WithContext(ctx), f, 500, cursorMs, cursorID, has, false)
				if err != nil {
					return err
				}
				for _, r := range rows {
					if sent >= limit {
						break
					}
					e := statusEntry(r)
					full := map[string]any{}
					raw, _ := json.Marshal(e)
					_ = json.Unmarshal(raw, &full)
					cells := []string{e.At.Format(time.RFC3339Nano), e.Level, e.Type, e.ServerName, e.MonitorName, e.Title, e.Detail}
					if err := emit(cells, full); err != nil {
						return err
					}
					sent++
				}
				if len(rows) < 500 {
					return nil
				}
				last := rows[len(rows)-1]
				cursorMs, cursorID, has = last.AtMs, last.ID, true
			}
			return nil
		},
	}
}
