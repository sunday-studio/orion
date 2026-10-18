package api

import (
	"context"
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/logstore"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const logSearchTimeout = 10 * time.Second

// scoped bounds a query's database work by the request and the 10 second search limit.
func scoped(c *gin.Context, q logstore.Query) (logstore.Query, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), logSearchTimeout)
	q.Ctx = ctx
	return q, cancel
}

// logQueryFailed answers a failed log query: 400 for a bad filter, 504 with a typed body for a timeout.
func logQueryFailed(c *gin.Context, err error, what string) {
	switch {
	case errors.Is(err, logstore.ErrBadFilter):
		utils.BadRequest(c, err.Error())
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "interrupted"):
		c.JSON(http.StatusGatewayTimeout, utils.APIResponse{Success: false, Message: "The search timed out. Narrow the time range or add a filter.", Data: SearchTimeoutBody{Code: "search_timeout"}})
	default:
		utils.InternalError(c, what, err)
	}
}

func (s *Server) appEnvironments(ids []string) map[string]string {
	env := map[string]string{}
	if len(ids) == 0 {
		return env
	}
	var apps []db.App
	s.db.Select("id", "environment").Where("id IN ?", ids).Find(&apps)
	for _, a := range apps {
		env[a.ID] = a.Environment
	}
	return env
}

func (s *Server) getLogFacetsImpl(c *gin.Context) {
	if !s.requireLogs(c) {
		return
	}
	q, _, _, ok := s.logQuery(c, time.Now().UTC())
	if !ok {
		return
	}
	q, cancel := scoped(c, q)
	defer cancel()
	topEvents := utils.QueryInt(c, "events", 5)
	if topEvents < 1 || topEvents > 50 {
		topEvents = 5
	}
	facets, err := s.logs.store.Facets(q, topEvents)
	if err != nil {
		logQueryFailed(c, err, "Failed to compute facets")
		return
	}
	ids := make([]string, 0, len(facets.Apps))
	for _, a := range facets.Apps {
		ids = append(ids, a.Value)
	}
	names, envs := s.appNames(ids), s.appEnvironments(ids)
	out := LogFacetsResponse{Total: facets.Total, Severity: facets.Severity, Sources: []LogFacetCount{}, Events: []LogFacetCount{}, EventsDistinct: facets.EventsDistinct, Environments: []LogFacetCount{}, Fields: []LogFieldStat{}}
	byEnv := map[string]int64{}
	for _, a := range facets.Apps {
		out.Sources = append(out.Sources, LogFacetCount{Value: a.Value, Label: names[a.Value], Count: a.Count})
		if env := envs[a.Value]; env != "" {
			byEnv[env] += a.Count
		}
	}
	for env, n := range byEnv {
		out.Environments = append(out.Environments, LogFacetCount{Value: env, Label: env, Count: n})
	}
	sortFacetCounts(out.Environments)
	for _, e := range facets.Events {
		out.Events = append(out.Events, LogFacetCount{Value: e.Value, Label: e.Value, Count: e.Count})
	}
	if len(q.AppIDs) == 1 {
		if stats, err := s.logs.store.FieldCatalog(q.AppIDs[0]); err == nil {
			for _, f := range stats {
				out.Fields = append(out.Fields, LogFieldStat{Key: f.Key, Type: f.Type, Cardinality: f.Cardinality, SampleValues: f.SampleValues, LastSeen: f.LastSeen})
			}
		}
	}
	if field := strings.TrimSpace(c.Query("field")); field != "" {
		values, err := s.logs.store.FieldValues(q, field, utils.QueryInt(c, "values", 10))
		if err != nil {
			logQueryFailed(c, err, "Failed to compute field values")
			return
		}
		for _, v := range values {
			out.Values = append(out.Values, LogFacetCount{Value: v.Value, Label: v.Value, Count: v.Count})
		}
	}
	utils.SuccessResponse(c, http.StatusOK, "Log facets retrieved successfully", out)
}

func sortFacetCounts(c []LogFacetCount) {
	for i := 1; i < len(c); i++ {
		for j := i; j > 0 && (c[j].Count > c[j-1].Count || (c[j].Count == c[j-1].Count && c[j].Value < c[j-1].Value)); j-- {
			c[j], c[j-1] = c[j-1], c[j]
		}
	}
}

func (s *Server) getLogPatternsImpl(c *gin.Context) {
	if !s.requireLogs(c) {
		return
	}
	now := time.Now().UTC()
	q, from, to, ok := s.logQuery(c, now)
	if !ok {
		return
	}
	if from.IsZero() {
		from = now.Add(-24 * time.Hour)
	}
	if to.IsZero() {
		to = now
	}
	q.From, q.To = from, to
	q, cancel := scoped(c, q)
	defer cancel()
	patterns, err := s.logs.store.Patterns(q, utils.QueryInt(c, "limit", 50))
	if err != nil {
		logQueryFailed(c, err, "Failed to compute patterns")
		return
	}
	out := make([]LogPattern, 0, len(patterns))
	for _, p := range patterns {
		out = append(out, LogPattern{Pattern: p.Template, Severity: LogSeverity(p.Severity), Count: p.Count, Trend: p.Trend, FirstSeen: p.FirstSeen, LastSeen: p.LastSeen, IsNew: p.New, SampleID: p.SampleID})
	}
	utils.SuccessResponse(c, http.StatusOK, "Log patterns retrieved successfully", LogPatternsResponse{Patterns: out, From: from, To: to})
}

func (s *Server) getLogRelatedImpl(c *gin.Context) {
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
	by, total, related, err := s.logs.store.Related(*entry, utils.QueryInt(c, "limit", 10))
	if err != nil {
		utils.InternalError(c, "Failed to load related logs", err)
		return
	}
	names := s.appNames([]string{entry.AppID})
	out := LogRelatedResponse{By: by, Total: total, Entries: make([]LogEntry, 0, len(related))}
	switch by {
	case "request_id":
		out.Value = entry.RequestID
	case "trace_id":
		out.Value = entry.TraceID
	}
	for _, e := range related {
		out.Entries = append(out.Entries, toLogEntry(e, names))
	}
	utils.SuccessResponse(c, http.StatusOK, "Related logs retrieved successfully", out)
}

func (s *Server) getLogIngestStatusImpl(c *gin.Context) {
	if !s.requireLogs(c) {
		return
	}
	now := time.Now().UTC()
	stats := s.logs.store.Stats()
	queued, queueMax := s.logs.store.QueueDepth()
	status := LogIngestStatus{QueueDepth: queued, QueueMax: queueMax, Dropped: stats.Dropped, StorageFull: stats.StorageFull, RateLimited: s.logs.ingester.RecentlyLimited(time.Minute)}
	if usage, err := s.logs.store.Usage(now); err == nil {
		status.LinesToday, status.StorageBytes = usage.TodayLines, usage.Bytes
	}
	// The writer drains about 20k entries a second on the target hardware; the lag estimate only needs to be honest, not exact.
	status.LagSeconds = int(queued/20000) + boolInt(queued > 0)
	status.Behind = status.StorageFull || queued*5 > queueMax || status.LagSeconds >= 30
	var apps []db.App
	s.db.Select("max_storage_mb").Find(&apps)
	var limit int64
	for _, a := range apps {
		if a.MaxStorageMB <= 0 {
			limit = 0
			break
		}
		limit += int64(a.MaxStorageMB) << 20
	}
	status.StorageLimitBytes = limit
	utils.SuccessResponse(c, http.StatusOK, "Log ingest status retrieved successfully", status)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
