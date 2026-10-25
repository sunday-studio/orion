package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/logstore"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// logQuery builds a logstore query from the shared filter parameters. It writes a
// 400 and returns false on bad input.
func (s *Server) logQuery(c *gin.Context, now time.Time) (logstore.Query, time.Time, time.Time, bool) {
	from, okFrom := parseLogTime(c.Query("from"), now)
	to, okTo := parseLogTime(c.Query("to"), now)
	if !okFrom || !okTo {
		utils.BadRequest(c, "from and to must be RFC 3339 times or relative offsets like -1h")
		return logstore.Query{}, from, to, false
	}
	q := logstore.Query{
		AppIDs: splitList(c.Query("app_id")), Severity: c.Query("severity"), Event: c.Query("event"), Text: c.Query("q"),
		From: from, To: to, TraceID: c.Query("trace_id"), RequestID: c.Query("request_id"), Attrs: c.QueryArray("attr"),
	}
	if c.GetBool(internalLogsKey) {
		q.AppIDs = []string{logstore.InternalAppID}
	}
	return q, from, to, true
}

func (s *Server) getLogHistogramImpl(c *gin.Context) {
	if !s.requireLogs(c) {
		return
	}
	now := time.Now().UTC()
	q, from, to, ok := s.logQuery(c, now)
	if !ok {
		return
	}
	if from.IsZero() {
		from = now.Add(-time.Hour)
	}
	if to.IsZero() {
		to = now
	}
	q, cancel := scoped(c, q)
	defer cancel()
	width, buckets, err := s.logs.store.Histogram(q, from, to, utils.QueryInt(c, "buckets", 60))
	if err != nil {
		logQueryFailed(c, err, "Failed to compute the histogram")
		return
	}
	out := make([]LogHistogramBucket, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, LogHistogramBucket{Start: b.Start, Total: b.Total, Severity: b.Severity})
	}
	utils.SuccessResponse(c, http.StatusOK, "Log histogram retrieved successfully", LogHistogramResponse{BucketSeconds: width, Buckets: out})
}

func (s *Server) getLogFieldCatalogImpl(c *gin.Context) {
	if !s.requireLogs(c) {
		return
	}
	appID := strings.TrimSpace(c.Query("app_id"))
	if appID == "" {
		utils.BadRequest(c, "app_id is required")
		return
	}
	stats, err := s.logs.store.FieldCatalog(appID)
	if err != nil {
		utils.InternalError(c, "Failed to build the field catalog", err)
		return
	}
	out := make([]LogFieldStat, 0, len(stats))
	for _, f := range stats {
		out = append(out, LogFieldStat{Key: f.Key, Type: f.Type, Cardinality: f.Cardinality, SampleValues: f.SampleValues, LastSeen: f.LastSeen})
	}
	utils.SuccessResponse(c, http.StatusOK, "Log field catalog retrieved successfully", LogFieldCatalogResponse{Fields: out})
}

// streamLogsImpl is a Server-Sent Events live tail.
func (s *Server) streamLogsImpl(c *gin.Context) {
	if !s.requireLogs(c) {
		return
	}
	q, _, _, ok := s.logQuery(c, time.Now().UTC())
	if !ok {
		return
	}
	q.From, q.To = time.Time{}, time.Time{}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		utils.InternalError(c, "Streaming is not supported by this connection", nil)
		return
	}
	controller := http.NewResponseController(c.Writer)

	sub, cancel := s.logs.store.Subscribe(q)
	defer cancel()
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	// The server-wide write timeout would cut a long-lived stream; each write extends this connection's deadline.
	extend := func() { _ = controller.SetWriteDeadline(time.Now().Add(time.Minute)) }
	send := func(e logstore.Entry) bool {
		payload, err := json.Marshal(toLogEntry(e, s.appNames([]string{e.AppID})))
		if err != nil {
			return true
		}
		extend()
		_, werr := fmt.Fprintf(c.Writer, "id: %s\nevent: log\ndata: %s\n\n", e.ID, payload)
		flusher.Flush()
		return werr == nil
	}

	// Resume after a reconnect: replay what the client missed, then go live.
	lastID := c.GetHeader("Last-Event-ID")
	var replayed string
	if lastID != "" {
		missed, err := s.logs.store.After(lastID, q, 500)
		if err == nil {
			for _, e := range missed {
				if !send(e) {
					return
				}
				replayed = e.ID
			}
		}
	}
	extend()
	fmt.Fprint(c.Writer, ": connected\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case e := <-sub.C:
			if replayed != "" && e.ID <= replayed {
				continue // already delivered during replay
			}
			if !send(e) {
				return
			}
		case <-heartbeat.C:
			extend()
			if _, err := fmt.Fprint(c.Writer, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) listLogViewsImpl(c *gin.Context) {
	var rows []db.LogSavedView
	if err := s.db.Order("created_at DESC").Find(&rows).Error; err != nil {
		utils.InternalError(c, "Failed to list saved views", err)
		return
	}
	out := make([]LogSavedView, 0, len(rows))
	for _, r := range rows {
		out = append(out, toSavedView(r))
	}
	utils.SuccessResponse(c, http.StatusOK, "Saved views retrieved successfully", gin.H{"views": out})
}

func toSavedView(r db.LogSavedView) LogSavedView {
	view := LogSavedView{ID: r.ID, Name: r.Name, CreatedAt: r.CreatedAt, Filters: map[string]string{}}
	_ = json.Unmarshal([]byte(r.FiltersJSON), &view.Filters)
	return view
}

var allowedViewFilters = map[string]bool{"app_id": true, "source": true, "severity": true, "event": true, "q": true, "from": true, "to": true, "trace_id": true, "request_id": true, "attr": true}

func (s *Server) createLogViewImpl(c *gin.Context) {
	var req LogSavedViewRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" || len(req.Name) > 80 {
		utils.BadRequest(c, "name is required (at most 80 characters) along with filters")
		return
	}
	for key, value := range req.Filters {
		if !allowedViewFilters[key] || len(value) > 500 {
			utils.BadRequest(c, fmt.Sprintf("filter %q is not a log search parameter", key))
			return
		}
	}
	encoded, _ := json.Marshal(req.Filters)
	row := db.LogSavedView{ID: utils.GenerateID("log_view"), Name: strings.TrimSpace(req.Name), FiltersJSON: string(encoded), CreatedBy: frontendActorID(c), CreatedAt: time.Now().UTC()}
	if err := s.db.Create(&row).Error; err != nil {
		utils.InternalError(c, "Failed to save the view", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "View saved", gin.H{"view": toSavedView(row)})
}

func (s *Server) deleteLogViewImpl(c *gin.Context) {
	res := s.db.Where("id = ?", c.Param("id")).Delete(&db.LogSavedView{})
	if res.Error != nil {
		utils.InternalError(c, "Failed to delete the view", res.Error)
		return
	}
	if res.RowsAffected == 0 {
		utils.NotFound(c, "Saved view not found")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "View deleted", nil)
}

// searchNodeLogs lists node service logs in the unified LogEntry shape.
func (s *Server) searchNodeLogs(c *gin.Context, from, to time.Time) {
	limit := min(utils.QueryInt(c, "limit", logstore.DefaultPageSize), logstore.MaxPageSize)
	query := s.db.Model(&db.ServiceLogEntry{})
	if nodeID := c.Query("node_id"); nodeID != "" {
		query = query.Where("node_id IN ?", splitList(nodeID))
	}
	if severity := c.Query("severity"); severity != "" {
		var levels []string
		for _, name := range splitList(strings.TrimPrefix(severity, ">=")) {
			canonical, _ := logstore.NormalizeSeverity(name)
			levels = append(levels, strings.ToUpper(canonical), canonical, strings.ToLower(name), strings.ToUpper(name))
			if canonical == "warn" {
				levels = append(levels, "WARNING", "warning")
			}
		}
		query = query.Where("level IN ?", levels)
	}
	if text := c.Query("q"); text != "" {
		query = query.Where("message LIKE ?", "%"+text+"%")
	}
	if !from.IsZero() {
		query = query.Where("occurred_at >= ?", from)
	}
	if !to.IsZero() {
		query = query.Where("occurred_at <= ?", to)
	}
	if cursor := c.Query("cursor"); cursor != "" {
		ts, id, err := logstore.DecodeCursor(cursor)
		if err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		at := time.Unix(0, ts).UTC()
		query = query.Where("(occurred_at < ? OR (occurred_at = ? AND id < ?))", at, at, id)
	}
	var rows []db.ServiceLogEntry
	if err := query.Order("occurred_at DESC, id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		utils.InternalError(c, "Failed to search node logs", err)
		return
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next = logstore.EncodeCursor(last.OccurredAt.UnixNano(), last.ID)
	}
	out := nodeLogEntries(rows)
	utils.SuccessResponse(c, http.StatusOK, "Logs retrieved successfully", LogListResponse{Entries: out, NextCursor: next})
}

func nodeLogEntries(rows []db.ServiceLogEntry) []LogEntry {
	out := make([]LogEntry, 0, len(rows))
	for _, r := range rows {
		severity, number := logstore.NormalizeSeverity(strings.ToLower(r.Level))
		attrs := map[string]interface{}{}
		_ = json.Unmarshal([]byte(r.FieldsJSON), &attrs)
		for k, v := range map[string]string{"component": r.Component, "monitor_name": r.MonitorName, "monitor_id": r.MonitorID, "stream": r.Stream, "source": r.Source} {
			if v != "" {
				attrs["node."+k] = v
			}
		}
		out = append(out, LogEntry{ID: r.ID, Source: "node", NodeID: r.NodeID, Timestamp: r.OccurredAt, IngestedAt: r.CollectedAt, Severity: LogSeverity(severity), SeverityValue: number, Body: r.Message, Attributes: attrs})
	}
	return out
}
