package api

import (
	"net/http"
	"orion/core/internal/utils"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	maxLogCountsWindow  = 7 * 24 * time.Hour
	maxLogCountsBuckets = 2000
)

// ServiceLogCountBucket is how many log entries the service's apps wrote in one bucket, by severity.
type ServiceLogCountBucket struct {
	T      time.Time        `json:"t"`
	Total  int64            `json:"total"`
	Counts map[string]int64 `json:"counts"`
}

// ServiceLogCountsResponse is the log volume of a service's apps over time, from the per-minute rollup.
type ServiceLogCountsResponse struct {
	ServiceID   string                  `json:"service_id"`
	From        time.Time               `json:"from"`
	To          time.Time               `json:"to"`
	StepSeconds int                     `json:"step_seconds"`
	AppIDs      []string                `json:"app_ids"`
	Buckets     []ServiceLogCountBucket `json:"buckets"`
}

func (s *Server) getServiceLogCountsImpl(c *gin.Context) {
	if !s.requireLogs(c) {
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
		from = to.Add(-time.Hour)
	}
	if !from.Before(to) {
		utils.BadRequest(c, "from must be before to")
		return
	}
	if to.Sub(from) > maxLogCountsWindow {
		utils.BadRequest(c, "The range is limited to 7 days")
		return
	}
	step := 60
	if raw := strings.TrimSpace(c.Query("step_seconds")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 60 || n > 3600 || n%60 != 0 {
			utils.BadRequest(c, "step_seconds must be a multiple of 60 between 60 and 3600")
			return
		}
		step = n
	}
	if int(to.Sub(from)/time.Second)/step > maxLogCountsBuckets {
		utils.BadRequest(c, "That range needs more than 2000 buckets: raise step_seconds or shorten the range")
		return
	}

	targets, err := s.serviceCatalog().MetricTargets(c.Param("id"))
	if err != nil {
		respondServiceError(c, err, "Failed to resolve the service's apps")
		return
	}
	out := ServiceLogCountsResponse{ServiceID: c.Param("id"), From: from, To: to, StepSeconds: step, AppIDs: targets.AppIDs, Buckets: []ServiceLogCountBucket{}}
	if out.AppIDs == nil {
		out.AppIDs = []string{}
	}
	rows, err := s.logs.store.MinuteCounts(targets.AppIDs, from, to)
	if err != nil {
		utils.InternalError(c, "Failed to read log counts", err)
		return
	}
	byBucket := map[int64]*ServiceLogCountBucket{}
	for _, r := range rows {
		at := r.Minute.Unix() / int64(step) * int64(step)
		b := byBucket[at]
		if b == nil {
			b = &ServiceLogCountBucket{T: time.Unix(at, 0).UTC(), Counts: map[string]int64{}}
			byBucket[at] = b
		}
		b.Counts[r.Severity] += r.Count
		b.Total += r.Count
	}
	for _, b := range byBucket {
		out.Buckets = append(out.Buckets, *b)
	}
	sort.Slice(out.Buckets, func(i, j int) bool { return out.Buckets[i].T.Before(out.Buckets[j].T) })
	utils.SuccessResponse(c, http.StatusOK, "Service log counts retrieved successfully", out)
}
