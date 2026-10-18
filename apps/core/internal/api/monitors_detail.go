package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type MonitorHeartbeatInfo struct {
	// Ping URLs show only the first characters of the token; the full URL is returned once, when the monitor is created.
	PingURLSuccess      string     `json:"ping_url_success"`
	PingURLFailure      string     `json:"ping_url_failure"`
	LastPingAt          *time.Time `json:"last_ping_at"`
	ExpectedEverySecond int        `json:"expected_every_seconds"`
	GraceSeconds        int        `json:"grace_seconds"`
}

// MonitorDetailSummary is the data beside the chart on the monitor detail screen.
type MonitorDetailSummary struct {
	Kind            string `json:"kind"`
	Target          string `json:"target,omitempty"`
	IntervalSeconds int    `json:"interval_seconds"`
	TimeoutSeconds  int    `json:"timeout_seconds,omitempty"`
	Status          string `json:"status" enums:"up,down,degraded,stale,unknown,paused"`
	StatusText      string `json:"status_text"`
	StatusDetail    string `json:"status_detail,omitempty"`
	// CurrentStateSince is when the monitor entered its current status (a failing run's start); null when unknown.
	CurrentStateSince *time.Time `json:"current_state_since"`
	LastCheckedAt     *time.Time `json:"last_checked_at"`
	Incidents30d      int64      `json:"incidents_30d"`
	RunbookURL        string     `json:"runbook_url,omitempty"`
	Paused            bool       `json:"paused"`
	PausedAt          *time.Time `json:"paused_at,omitempty"`
	PausedBy          string     `json:"paused_by,omitempty"`
	// LastResult is the newest check: pass or fail.
	LastResult string `json:"last_result,omitempty" enums:"pass,fail"`
	// TLSDaysRemaining comes from the newest TLS-aware check.
	TLSDaysRemaining *int                  `json:"tls_days_remaining,omitempty"`
	Heartbeat        *MonitorHeartbeatInfo `json:"heartbeat,omitempty"`
}

func (s *Server) monitorDetailSummary(c *gin.Context, monitor db.Monitor, health string) MonitorDetailSummary {
	rows := []MonitorResponse{monitorResponse(monitor)}
	s.enrichMonitorRows(rows, []db.Monitor{monitor})
	r := rows[0]
	sum := MonitorDetailSummary{Kind: r.Kind, Target: r.Target, IntervalSeconds: r.IntervalSeconds, Status: health, StatusText: r.StatusText, StatusDetail: r.StatusDetail,
		RunbookURL: monitor.RunbookURL, Paused: r.Paused, PausedAt: r.PausedAt, PausedBy: r.PausedBy}
	if r.Paused {
		sum.Status = "paused"
	}
	checks := s.recentMonitorChecks([]string{monitor.ID})[monitor.ID]
	if len(checks) > 0 {
		t := checks[0].createdAt
		sum.LastCheckedAt = &t
		sum.LastResult = map[bool]string{true: "pass", false: "fail"}[checks[0].health == "up"]
		sum.CurrentStateSince, _ = failureFromChecks(checks)
		if v := tlsDaysFromPayload(checks[0].payload); v != nil {
			sum.TLSDaysRemaining = v
		}
	}
	s.db.Model(&db.Incident{}).Where("monitor_id = ? AND opened_at >= ?", monitor.ID, time.Now().UTC().AddDate(0, 0, -30)).Count(&sum.Incidents30d)
	var cfg db.CoreMonitorConfig
	if s.db.Where("monitor_id = ?", monitor.ID).Limit(1).Find(&cfg).RowsAffected > 0 {
		sum.TimeoutSeconds = cfg.TimeoutSeconds
		if cfg.Kind == "heartbeat" {
			base := s.coreURLFor(c) + "/v1/heartbeats/" + cfg.HeartbeatTokenPrefix + "..."
			grace := 0
			if v, ok := monitorConfigMap(cfg)["grace_seconds"].(float64); ok {
				grace = int(v)
			}
			sum.Heartbeat = &MonitorHeartbeatInfo{PingURLSuccess: base + "/success", PingURLFailure: base + "/failure", LastPingAt: cfg.LastSignalAt, ExpectedEverySecond: cfg.IntervalSeconds, GraceSeconds: grace}
		}
	}
	return sum
}

// ResponseTimePoint is one bucket of the response time chart.
type ResponseTimePoint struct {
	At     time.Time `json:"at"`
	AvgMs  *float64  `json:"avg_ms"`
	MaxMs  *float64  `json:"max_ms"`
	Checks int64     `json:"checks"`
	Failed int64     `json:"failed"`
}

type ResponseTimesResponse struct {
	Range         string              `json:"range" enums:"day,week,month"`
	BucketSeconds int                 `json:"bucket_seconds"`
	TimeoutMs     int                 `json:"timeout_ms,omitempty"`
	Points        []ResponseTimePoint `json:"points"`
}

// getMonitorResponseTimes returns bucketed response times for the chart.
// @Summary      Monitor response times
// @Description  Average and maximum check duration per bucket: day = 5 minute buckets, week = hourly, month = 6 hourly. Empty buckets are omitted.
// @Tags         monitors
// @Produce      json
// @ID           getMonitorResponseTimes
// @Param        id     path      string  true   "Monitor ID"
// @Param        range  query     string  false  "day, week or month" default(day)
// @Success      200    {object}  utils.APIResponse{data=ResponseTimesResponse}
// @Failure      404    {object}  utils.APIResponse
// @Router       /v1/monitors/{id}/response-times [get]
func (s *Server) getMonitorResponseTimes(c *gin.Context) {
	if _, err := s.monitorService.GetMonitor(c.Param("id")); err != nil {
		utils.NotFound(c, "Monitor not found")
		return
	}
	span, bucket, name := 24*time.Hour, 300, "day"
	switch c.DefaultQuery("range", "day") {
	case "week":
		span, bucket, name = 7*24*time.Hour, 3600, "week"
	case "month":
		span, bucket, name = 30*24*time.Hour, 6*3600, "month"
	}
	var rows []struct {
		Bucket int64
		Avg    *float64
		Max    *float64
		Checks int64
		Failed int64
	}
	since := time.Now().UTC().Add(-span)
	s.db.Raw(`SELECT (CAST(strftime('%s', created_at) AS INTEGER) / ?) * ? AS bucket,
		AVG(json_extract(payload, '$.duration_ms')) AS avg, MAX(json_extract(payload, '$.duration_ms')) AS max,
		COUNT(*) AS checks, SUM(CASE WHEN health <> 'up' THEN 1 ELSE 0 END) AS failed
		FROM monitor_reports WHERE monitor_id = ? AND created_at >= ? GROUP BY bucket ORDER BY bucket`, bucket, bucket, c.Param("id"), since).Scan(&rows)
	out := ResponseTimesResponse{Range: name, BucketSeconds: bucket, Points: make([]ResponseTimePoint, 0, len(rows))}
	var cfg db.CoreMonitorConfig
	if s.db.Where("monitor_id = ?", c.Param("id")).Limit(1).Find(&cfg).RowsAffected > 0 {
		out.TimeoutMs = cfg.TimeoutSeconds * 1000
	}
	for _, r := range rows {
		out.Points = append(out.Points, ResponseTimePoint{At: time.Unix(r.Bucket, 0).UTC(), AvgMs: r.Avg, MaxMs: r.Max, Checks: r.Checks, Failed: r.Failed})
	}
	utils.SuccessResponse(c, http.StatusOK, "Response times retrieved", out)
}

type AvailabilityPeriod struct {
	Period          string   `json:"period" enums:"today,7d,30d,365d"`
	AvailabilityPct *float64 `json:"availability_percent"`
	DowntimeSeconds int64    `json:"downtime_seconds"`
	Incidents       int64    `json:"incidents"`
	LongestSeconds  int64    `json:"longest_seconds"`
	// LongestOngoing is true when the longest incident has not ended.
	LongestOngoing bool `json:"longest_ongoing"`
}

// getMonitorAvailability returns availability and incident stats for four periods.
// @Summary      Monitor availability
// @Description  Per period: check-based availability, incident-based downtime, incident count and the longest incident.
// @Tags         monitors
// @Produce      json
// @ID           getMonitorAvailability
// @Param        id   path      string  true  "Monitor ID"
// @Success      200  {object}  utils.APIResponse{data=object{periods=[]AvailabilityPeriod}}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/monitors/{id}/availability [get]
func (s *Server) getMonitorAvailability(c *gin.Context) {
	id := c.Param("id")
	if _, err := s.monitorService.GetMonitor(id); err != nil {
		utils.NotFound(c, "Monitor not found")
		return
	}
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	periods := []struct {
		name  string
		start time.Time
	}{{"today", today}, {"7d", now.AddDate(0, 0, -7)}, {"30d", now.AddDate(0, 0, -30)}, {"365d", now.AddDate(0, 0, -365)}}
	var incidents []db.Incident
	s.db.Where("monitor_id = ? AND (resolved_at IS NULL OR resolved_at >= ?)", id, periods[3].start).Find(&incidents)
	out := make([]AvailabilityPeriod, 0, len(periods))
	for _, p := range periods {
		ap := AvailabilityPeriod{Period: p.name}
		for _, inc := range incidents {
			end := now
			if inc.ResolvedAt != nil {
				end = *inc.ResolvedAt
			}
			start := inc.OpenedAt
			if start.Before(p.start) {
				start = p.start
			}
			if !end.After(start) {
				continue
			}
			ap.Incidents++
			d := int64(end.Sub(start).Seconds())
			ap.DowntimeSeconds += d
			if d > ap.LongestSeconds {
				ap.LongestSeconds, ap.LongestOngoing = d, inc.ResolvedAt == nil
			}
		}
		var up, total int64
		rollupFrom := p.start.Format("2006-01-02")
		s.db.Model(&db.MonitorUptimeRollup{}).Select("COALESCE(SUM(up_count),0)").Where("monitor_id = ? AND date >= ? AND date < ?", id, rollupFrom, today.Format("2006-01-02")).Scan(&up)
		s.db.Model(&db.MonitorUptimeRollup{}).Select("COALESCE(SUM(total_count),0)").Where("monitor_id = ? AND date >= ? AND date < ?", id, rollupFrom, today.Format("2006-01-02")).Scan(&total)
		var liveUp, liveTotal int64
		liveFrom := today
		if p.start.After(today) {
			liveFrom = p.start
		}
		s.db.Model(&db.MonitorReport{}).Where("monitor_id = ? AND created_at >= ?", id, liveFrom).Count(&liveTotal)
		s.db.Model(&db.MonitorReport{}).Where("monitor_id = ? AND created_at >= ? AND health = ?", id, liveFrom, "up").Count(&liveUp)
		up, total = up+liveUp, total+liveTotal
		if total > 0 {
			v := 100 * float64(up) / float64(total)
			ap.AvailabilityPct = &v
		}
		out = append(out, ap)
	}
	utils.SuccessResponse(c, http.StatusOK, "Availability retrieved", gin.H{"periods": out})
}

// cloneCoreMonitor duplicates a Core monitor.
// @Summary      Clone a Core monitor
// @Description  Creates a copy with " copy" appended to the name. Heartbeat clones get their own token, shown once.
// @Tags         monitors
// @Produce      json
// @ID           cloneCoreMonitor
// @Param        id   path      string  true  "Monitor ID"
// @Success      201  {object}  utils.APIResponse{data=CoreMonitorManagementResponse}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/monitors/{id}/clone [post]
func (s *Server) cloneCoreMonitor(c *gin.Context) {
	record, err := s.coreMonitorManagementService.CloneCoreMonitor(c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrCoreManagedMonitorNotFound) {
			utils.NotFound(c, "Monitor not found")
			return
		}
		s.handleCoreMonitorManagementError(c, err, "Failed to clone monitor")
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Monitor cloned", s.coreMonitorManagementResponse(record))
}

func tlsDaysFromPayload(payload string) *int {
	if !strings.Contains(payload, "tls_days_remaining") {
		return nil
	}
	var m map[string]any
	if json.Unmarshal([]byte(payload), &m) != nil {
		return nil
	}
	if v, ok := m["tls_days_remaining"].(float64); ok {
		d := int(v)
		return &d
	}
	return nil
}
