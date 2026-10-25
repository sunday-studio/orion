package api

import (
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/logstore"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func splitList(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (s *Server) appNames(ids []string) map[string]string {
	names := map[string]string{}
	if len(ids) == 0 {
		return names
	}
	var apps []db.App
	s.db.Select("id", "name").Where("id IN ?", ids).Find(&apps)
	for _, app := range apps {
		names[app.ID] = app.Name
	}
	return names
}

func toLogEntry(e logstore.Entry, appNames map[string]string) LogEntry {
	out := LogEntry{
		ID: e.ID, Source: "app", AppID: e.AppID, AppName: appNames[e.AppID], Timestamp: e.Timestamp, IngestedAt: e.IngestedAt,
		Severity: LogSeverity(e.Severity), SeverityValue: e.SeverityNumber, SeverityInferred: e.SeverityInferred, Event: e.Event, Body: e.Body,
		Attributes: e.Attributes, Resource: e.Resource, TraceID: e.TraceID, SpanID: e.SpanID, RequestID: e.RequestID,
	}
	if e.AppID == logstore.InternalAppID {
		out.Source, out.AppName = "internal", "Orion Core"
	}
	if e.BodyJSON != "" {
		var structured interface{}
		if err := jsonUnmarshal(e.BodyJSON, &structured); err == nil {
			out.BodyJSON = structured
		}
	}
	return out
}

func (s *Server) searchLogsImpl(c *gin.Context) {
	if !s.requireLogs(c) {
		return
	}
	now := time.Now().UTC()
	from, okFrom := parseLogTime(c.Query("from"), now)
	to, okTo := parseLogTime(c.Query("to"), now)
	if !okFrom || !okTo {
		utils.BadRequest(c, "from and to must be RFC 3339 times or relative offsets like -1h")
		return
	}
	var appIDs []string
	appIDs = append(appIDs, splitList(c.Query("app_id"))...)
	internal := c.GetBool(internalLogsKey)
	if internal {
		appIDs = []string{logstore.InternalAppID}
	}

	if source := c.Query("source"); source == "node" && !internal {
		s.searchNodeLogs(c, from, to)
		return
	}

	query, cancel := scoped(c, logstore.Query{
		AppIDs: appIDs, Severity: c.Query("severity"), Event: c.Query("event"), Text: c.Query("q"), From: from, To: to,
		TraceID: c.Query("trace_id"), RequestID: c.Query("request_id"), Attrs: c.QueryArray("attr"),
		Limit: utils.QueryInt(c, "limit", logstore.DefaultPageSize), Cursor: c.Query("cursor"),
	})
	defer cancel()
	entries, next, err := s.logs.store.List(query)
	if err != nil {
		logQueryFailed(c, err, "Failed to search logs")
		return
	}
	ids := make([]string, 0, len(entries))
	seen := map[string]bool{}
	for _, e := range entries {
		if !seen[e.AppID] {
			seen[e.AppID] = true
			ids = append(ids, e.AppID)
		}
	}
	names := s.appNames(ids)
	out := make([]LogEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, toLogEntry(e, names))
	}
	utils.SuccessResponse(c, http.StatusOK, "Logs retrieved successfully", LogListResponse{Entries: out, NextCursor: next})
}

func (s *Server) getLogEntryImpl(c *gin.Context) {
	if !s.requireLogs(c) {
		return
	}
	entry, err := s.logs.store.Get(c.Param("id"))
	if err != nil {
		utils.InternalError(c, "Failed to load the log entry", err)
		return
	}
	if entry == nil {
		utils.NotFound(c, "Log entry not found")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Log entry retrieved successfully", gin.H{"entry": toLogEntry(*entry, s.appNames([]string{entry.AppID}))})
}
