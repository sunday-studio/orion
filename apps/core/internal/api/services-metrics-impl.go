package api

import (
	"net/http"
	"orion/core/internal/metricstore"
	"orion/core/internal/utils"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultMetricsWindow = time.Hour
	maxMetricsWindow     = 90 * 24 * time.Hour
)

func (s *Server) getServiceMetricsImpl(c *gin.Context) {
	if s.metrics == nil {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Metric history is not enabled on this Core instance", nil)
		return
	}
	now := time.Now().UTC()
	to, okTo := parseLogTime(strings.TrimSpace(c.Query("to")), now)
	from, okFrom := parseLogTime(strings.TrimSpace(c.Query("from")), now)
	if !okTo || !okFrom {
		utils.BadRequest(c, "from and to must be RFC 3339 times or relative offsets such as -1h")
		return
	}
	if to.IsZero() {
		to = now
	}
	if from.IsZero() {
		from = to.Add(-defaultMetricsWindow)
	}
	if !from.Before(to) {
		utils.BadRequest(c, "from must be before to")
		return
	}
	if to.Sub(from) > maxMetricsWindow {
		utils.BadRequest(c, "The range is limited to 90 days")
		return
	}
	maxPoints := 0
	if raw := strings.TrimSpace(c.Query("max_points")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			utils.BadRequest(c, "max_points must be a positive number")
			return
		}
		maxPoints = n
	}

	targets, err := s.serviceCatalog().MetricTargets(c.Param("id"))
	if err != nil {
		respondServiceError(c, err, "Failed to resolve the service's metrics")
		return
	}
	out := ServiceMetricsResponse{ServiceID: c.Param("id"), From: from, To: to, Series: []ServiceMetricSeries{}}
	result, err := s.metrics.Query(metricstore.Query{Series: targets.Resources, AppIDs: targets.AppIDs, From: from, To: to, MaxPoints: maxPoints})
	if err != nil {
		utils.InternalError(c, "Failed to read metric history", err)
		return
	}
	out.Resolution, out.StepSeconds, out.Truncated = result.Resolution, result.StepSeconds, result.Truncated
	for _, sr := range result.Series {
		series := ServiceMetricSeries{Source: sr.Ref.Source, NodeID: sr.Ref.NodeID, AppID: sr.Ref.AppID, Key: sr.Ref.Key, Kind: sr.Kind, Name: sr.Name,
			Metric: sr.Metric, Unit: sr.Unit, Labels: sr.Labels, Points: make([]ServiceMetricPoint, 0, len(sr.Points))}
		for _, p := range sr.Points {
			series.Points = append(series.Points, ServiceMetricPoint{T: p.T, CPUPercent: p.CPUPercent, CPUPercentMax: p.CPUPercentMax, MemoryBytes: p.MemoryBytes,
				MemoryBytesMax: p.MemoryBytesMax, IOReadBps: p.IOReadBps, IOWriteBps: p.IOWriteBps, NetRxBps: p.NetRxBps, NetTxBps: p.NetTxBps,
				PidsMax: p.PidsMax, StorageMax: p.StorageMax, Value: p.Value, ValueMax: p.ValueMax})
		}
		out.Series = append(out.Series, series)
	}
	utils.SuccessResponse(c, http.StatusOK, "Service metrics retrieved successfully", out)
}
