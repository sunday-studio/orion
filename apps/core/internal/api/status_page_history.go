package api

import (
	"fmt"
	"math"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	statusPagePublicDefaultUptimeWindow = "90d"
	statusPagePublicNoDataDisplay       = "No data"
)

type StatusPagePublicUptimeResponse struct {
	Window        string   `json:"window"`
	UptimeRatio   *float64 `json:"uptime_ratio"`
	UptimeDisplay string   `json:"uptime_display"`
}

type StatusPagePublicUptimeBucketResponse struct {
	Date          string   `json:"date"`
	UptimeRatio   *float64 `json:"uptime_ratio"`
	UptimeDisplay string   `json:"uptime_display"`
}

type StatusPagePublicComponentHistoryResponse struct {
	Component StatusPagePublicComponentResponse      `json:"component"`
	Uptime    StatusPagePublicUptimeResponse         `json:"uptime"`
	History   []StatusPagePublicUptimeBucketResponse `json:"history,omitempty"`
}

type StatusPagePublicHistoryResponse struct {
	Page        StatusPagePublicPageResponse               `json:"page"`
	Components  []StatusPagePublicComponentHistoryResponse `json:"components"`
	Incidents   []StatusPagePublicIncidentResponse         `json:"incidents"`
	GeneratedAt time.Time                                  `json:"generated_at"`
}

type StatusPagePublicIncidentUpdateHistoryResponse struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	Message     string     `json:"message"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

type StatusPagePublicIncidentHistoryResponse struct {
	Incident StatusPagePublicIncidentResponse                `json:"incident"`
	Updates  []StatusPagePublicIncidentUpdateHistoryResponse `json:"updates"`
}

type statusPagePublicResourceUptime struct {
	buckets       []service.UptimeDayBucket
	totalUp       int
	totalSamples  int
	uptimePercent float64
}

// getPublicStatusPageHistory returns public incident and component uptime history.
// @Summary      Get public status page history
// @Description  Get sanitized public history for a published or unlisted status page
// @Tags         public-status
// @Accept       json
// @Produce      json
// @ID           getPublicStatusPageHistory
// @Param        slug    path   string  true   "Status page slug"
// @Param        window  query  string  false  "Uptime window: 24h, 7d, 30d, or 90d" default(90d)
// @Success      200     {object}  utils.APIResponse{data=object{history=StatusPagePublicHistoryResponse}}
// @Failure      400     {object}  utils.APIResponse
// @Failure      404     {object}  utils.APIResponse
// @Failure      500     {object}  utils.APIResponse
// @Router       /status/{slug}/history [get]
func (s *Server) getPublicStatusPageHistory(c *gin.Context) {
	window, ok := publicStatusPageWindow(c)
	if !ok {
		return
	}
	detail, ok := s.loadPublicStatusPageDetail(c, c.Param("slug"))
	if !ok {
		return
	}
	preview := s.statusPagePreview(detail, false)
	utils.SuccessResponse(c, http.StatusOK, "Status page history retrieved successfully", gin.H{
		"history": StatusPagePublicHistoryResponse{
			Page:        preview.Page,
			Components:  s.publicStatusPageComponentHistories(detail, window, false),
			Incidents:   preview.Incidents,
			GeneratedAt: publicMinute(time.Now()),
		},
	})
}

// getPublicStatusPageComponentUptime returns a public component uptime summary.
// @Summary      Get public status page component uptime
// @Description  Get a sanitized public uptime summary for one visible component
// @Tags         public-status
// @Accept       json
// @Produce      json
// @ID           getPublicStatusPageComponentUptime
// @Param        slug          path   string  true   "Status page slug"
// @Param        component_id  path   string  true   "Public component ID"
// @Param        window        query  string  false  "Uptime window: 24h, 7d, 30d, or 90d" default(90d)
// @Success      200           {object}  utils.APIResponse{data=object{component=StatusPagePublicComponentResponse,uptime=StatusPagePublicUptimeResponse}}
// @Failure      400           {object}  utils.APIResponse
// @Failure      404           {object}  utils.APIResponse
// @Failure      500           {object}  utils.APIResponse
// @Router       /status/{slug}/components/{component_id}/uptime [get]
func (s *Server) getPublicStatusPageComponentUptime(c *gin.Context) {
	window, ok := publicStatusPageWindow(c)
	if !ok {
		return
	}
	component, ok := s.publicStatusPageComponentForRequest(c)
	if !ok {
		return
	}
	uptime, _ := s.publicStatusPageComponentUptime(component, window)
	utils.SuccessResponse(c, http.StatusOK, "Status page component uptime retrieved successfully", gin.H{
		"component": s.statusPagePublicComponentResponse(component, s.statusPageComponentStatus(component)),
		"uptime":    uptime,
	})
}

// getPublicStatusPageComponentHistory returns public component uptime history.
// @Summary      Get public status page component history
// @Description  Get sanitized public uptime history for one visible component
// @Tags         public-status
// @Accept       json
// @Produce      json
// @ID           getPublicStatusPageComponentHistory
// @Param        slug          path   string  true   "Status page slug"
// @Param        component_id  path   string  true   "Public component ID"
// @Param        window        query  string  false  "Uptime window: 24h, 7d, 30d, or 90d" default(90d)
// @Success      200           {object}  utils.APIResponse{data=object{component=StatusPagePublicComponentResponse,uptime=StatusPagePublicUptimeResponse,history=[]StatusPagePublicUptimeBucketResponse}}
// @Failure      400           {object}  utils.APIResponse
// @Failure      404           {object}  utils.APIResponse
// @Failure      500           {object}  utils.APIResponse
// @Router       /status/{slug}/components/{component_id}/history [get]
func (s *Server) getPublicStatusPageComponentHistory(c *gin.Context) {
	window, ok := publicStatusPageWindow(c)
	if !ok {
		return
	}
	component, ok := s.publicStatusPageComponentForRequest(c)
	if !ok {
		return
	}
	uptime, history := s.publicStatusPageComponentUptime(component, window)
	utils.SuccessResponse(c, http.StatusOK, "Status page component history retrieved successfully", gin.H{
		"component": s.statusPagePublicComponentResponse(component, s.statusPageComponentStatus(component)),
		"uptime":    uptime,
		"history":   history,
	})
}

// getPublicStatusPageIncidentHistory returns public incident update history.
// @Summary      Get public status page incident history
// @Description  Get published public updates for one public incident
// @Tags         public-status
// @Accept       json
// @Produce      json
// @ID           getPublicStatusPageIncidentHistory
// @Param        slug         path  string  true  "Status page slug"
// @Param        incident_id  path  string  true  "Public incident ID"
// @Success      200          {object}  utils.APIResponse{data=object{history=StatusPagePublicIncidentHistoryResponse}}
// @Failure      404          {object}  utils.APIResponse
// @Failure      500          {object}  utils.APIResponse
// @Router       /status/{slug}/incidents/{incident_id}/history [get]
func (s *Server) getPublicStatusPageIncidentHistory(c *gin.Context) {
	preview, ok := s.loadPublicStatusPageProjection(c, c.Param("slug"))
	if !ok {
		return
	}
	var incident StatusPagePublicIncidentResponse
	for _, candidate := range preview.Incidents {
		if candidate.ID == c.Param("incident_id") {
			incident = candidate
			break
		}
	}
	if incident.ID == "" {
		utils.NotFound(c, "Status page incident not found")
		return
	}

	updates, err := s.publicStatusPageIncidentUpdates(incident.ID)
	if err != nil {
		s.logger.Error("Failed to load public status page incident history", "incident_id", incident.ID, "error", err)
		utils.InternalError(c, "Failed to load status page incident history", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Status page incident history retrieved successfully", gin.H{
		"history": StatusPagePublicIncidentHistoryResponse{
			Incident: incident,
			Updates:  updates,
		},
	})
}

func publicStatusPageWindow(c *gin.Context) (string, bool) {
	window := strings.TrimSpace(c.Query("window"))
	if window == "" {
		return statusPagePublicDefaultUptimeWindow, true
	}
	switch window {
	case "24h", "7d", "30d", "90d":
		return window, true
	default:
		utils.BadRequest(c, "Unsupported uptime window")
		return "", false
	}
}

func (s *Server) publicStatusPageComponentForRequest(c *gin.Context) (StatusPageComponentResponse, bool) {
	detail, ok := s.loadPublicStatusPageDetail(c, c.Param("slug"))
	if !ok {
		return StatusPageComponentResponse{}, false
	}
	componentID := c.Param("component_id")
	for _, component := range detail.Components {
		if component.ID == componentID && component.Visible {
			return component, true
		}
	}
	utils.NotFound(c, "Status page component not found")
	return StatusPageComponentResponse{}, false
}

func (s *Server) publicStatusPageComponentHistories(detail StatusPageDetailResponse, window string, includeBuckets bool) []StatusPagePublicComponentHistoryResponse {
	responses := make([]StatusPagePublicComponentHistoryResponse, 0, len(detail.Components))
	for _, component := range detail.Components {
		if !component.Visible {
			continue
		}
		uptime, history := s.publicStatusPageComponentUptime(component, window)
		if !includeBuckets {
			history = nil
		}
		responses = append(responses, StatusPagePublicComponentHistoryResponse{
			Component: s.statusPagePublicComponentResponse(component, s.statusPageComponentStatus(component)),
			Uptime:    uptime,
			History:   history,
		})
	}
	return responses
}

func (s *Server) publicStatusPageComponentUptime(component StatusPageComponentResponse, window string) (StatusPagePublicUptimeResponse, []StatusPagePublicUptimeBucketResponse) {
	if len(component.Mappings) == 0 {
		return noDataPublicUptime(window), publicEmptyUptimeHistory(window)
	}

	resourceUptimes := make([]statusPagePublicResourceUptime, 0, len(component.Mappings))
	for _, mapping := range component.Mappings {
		uptime, ok := s.publicStatusPageMappedResourceUptime(mapping, window)
		if ok {
			resourceUptimes = append(resourceUptimes, uptime)
		}
	}
	if len(resourceUptimes) == 0 {
		return noDataPublicUptime(window), publicEmptyUptimeHistory(window)
	}

	strategy := publicComponentUptimeStrategy(component.Mappings)
	ratio := aggregatePublicUptimeRatio(resourceUptimes, strategy)
	return publicUptime(window, ratio), aggregatePublicUptimeHistory(resourceUptimes, strategy, window)
}

func (s *Server) publicStatusPageMappedResourceUptime(mapping StatusPageComponentMappingResponse, window string) (statusPagePublicResourceUptime, bool) {
	period := publicWindowReportPeriod(window)
	var result *service.UptimeResult
	var err error
	switch mapping.ResourceType {
	case "monitor":
		result, err = s.reportService.GetMonitorUptime(mapping.ResourceID, period)
	case "agent":
		result, err = s.reportService.GetAgentUptime(mapping.ResourceID, period)
	default:
		return statusPagePublicResourceUptime{}, false
	}
	if err != nil {
		s.logger.Warn("Failed to compute public status page mapped resource uptime", "resource_type", mapping.ResourceType, "error", err)
		return statusPagePublicResourceUptime{}, false
	}

	if result.TotalCount == 0 {
		return statusPagePublicResourceUptime{}, false
	}
	return statusPagePublicResourceUptime{
		buckets:       result.DailyBuckets,
		totalUp:       result.UpCount,
		totalSamples:  result.TotalCount,
		uptimePercent: result.UptimePercent,
	}, true
}

func publicComponentUptimeStrategy(mappings []StatusPageComponentMappingResponse) string {
	for _, mapping := range mappings {
		switch mapping.UptimeRollupStrategy {
		case "average", "best", "worst":
			return mapping.UptimeRollupStrategy
		}
	}
	return "worst"
}

func aggregatePublicUptimeRatio(resources []statusPagePublicResourceUptime, strategy string) *float64 {
	if len(resources) == 0 {
		return nil
	}
	switch strategy {
	case "average":
		up, total := 0, 0
		for _, resource := range resources {
			up += resource.totalUp
			total += resource.totalSamples
		}
		return publicRatio(up, total)
	case "best":
		best := -1.0
		for _, resource := range resources {
			ratio := resource.uptimePercent / 100
			if best < 0 || ratio > best {
				best = ratio
			}
		}
		return &best
	default:
		worst := 2.0
		for _, resource := range resources {
			ratio := resource.uptimePercent / 100
			if ratio < worst {
				worst = ratio
			}
		}
		return &worst
	}
}

func aggregatePublicUptimeHistory(resources []statusPagePublicResourceUptime, strategy string, window string) []StatusPagePublicUptimeBucketResponse {
	if len(resources) == 0 {
		return publicEmptyUptimeHistory(window)
	}

	base := resources[0].buckets
	history := make([]StatusPagePublicUptimeBucketResponse, 0, len(base))
	for index, bucket := range base {
		ratio := aggregatePublicUptimeBucketRatio(resources, strategy, index)
		history = append(history, publicUptimeBucket(bucket.Date, ratio))
	}
	return history
}

func aggregatePublicUptimeBucketRatio(resources []statusPagePublicResourceUptime, strategy string, index int) *float64 {
	switch strategy {
	case "average":
		up, total := 0, 0
		for _, resource := range resources {
			if index >= len(resource.buckets) {
				continue
			}
			up += resource.buckets[index].Up
			total += resource.buckets[index].Total
		}
		return publicRatio(up, total)
	case "best":
		best := -1.0
		for _, resource := range resources {
			if index >= len(resource.buckets) || resource.buckets[index].Total == 0 {
				continue
			}
			ratio := float64(resource.buckets[index].Up) / float64(resource.buckets[index].Total)
			if best < 0 || ratio > best {
				best = ratio
			}
		}
		if best < 0 {
			return nil
		}
		return &best
	default:
		worst := 2.0
		for _, resource := range resources {
			if index >= len(resource.buckets) || resource.buckets[index].Total == 0 {
				continue
			}
			ratio := float64(resource.buckets[index].Up) / float64(resource.buckets[index].Total)
			if ratio < worst {
				worst = ratio
			}
		}
		if worst > 1 {
			return nil
		}
		return &worst
	}
}

func publicRatio(up int, total int) *float64 {
	if total <= 0 {
		return nil
	}
	ratio := float64(up) / float64(total)
	return &ratio
}

func publicUptime(window string, ratio *float64) StatusPagePublicUptimeResponse {
	return StatusPagePublicUptimeResponse{
		Window:        window,
		UptimeRatio:   publicRoundedRatio(ratio),
		UptimeDisplay: publicUptimeDisplay(ratio),
	}
}

func noDataPublicUptime(window string) StatusPagePublicUptimeResponse {
	return publicUptime(window, nil)
}

func publicUptimeBucket(date string, ratio *float64) StatusPagePublicUptimeBucketResponse {
	return StatusPagePublicUptimeBucketResponse{
		Date:          date,
		UptimeRatio:   publicRoundedRatio(ratio),
		UptimeDisplay: publicUptimeDisplay(ratio),
	}
}

func publicRoundedRatio(ratio *float64) *float64 {
	if ratio == nil {
		return nil
	}
	rounded := math.Round(*ratio*10000) / 10000
	return &rounded
}

func publicUptimeDisplay(ratio *float64) string {
	if ratio == nil {
		return statusPagePublicNoDataDisplay
	}
	percent := *ratio * 100
	if math.Abs(percent-100) < 0.005 {
		return "100%"
	}
	if percent >= 99 {
		return fmt.Sprintf("%.2f%%", percent)
	}
	return fmt.Sprintf("%.1f%%", percent)
}

func publicEmptyUptimeHistory(window string) []StatusPagePublicUptimeBucketResponse {
	days := publicWindowDays(window)
	since := time.Now().UTC().AddDate(0, 0, -days)
	history := make([]StatusPagePublicUptimeBucketResponse, 0, days)
	for day := 0; day < days; day++ {
		date := since.AddDate(0, 0, day).Format("2006-01-02")
		history = append(history, publicUptimeBucket(date, nil))
	}
	return history
}

func publicWindowReportPeriod(window string) string {
	if window == "24h" {
		return "1d"
	}
	return window
}

func publicWindowDays(window string) int {
	switch window {
	case "24h":
		return 1
	case "7d":
		return 7
	case "30d":
		return 30
	default:
		return 90
	}
}

func (s *Server) publicStatusPageIncidentUpdates(incidentID string) ([]StatusPagePublicIncidentUpdateHistoryResponse, error) {
	var updates []db.StatusPageIncidentUpdate
	if err := s.db.
		Where("incident_id = ? AND published_at IS NOT NULL", incidentID).
		Order("published_at ASC").
		Find(&updates).Error; err != nil {
		return nil, err
	}
	responses := make([]StatusPagePublicIncidentUpdateHistoryResponse, 0, len(updates))
	for _, update := range updates {
		responses = append(responses, StatusPagePublicIncidentUpdateHistoryResponse{
			ID:          update.ID,
			Status:      update.Status,
			Message:     update.Message,
			PublishedAt: publicMinutePtr(update.PublishedAt),
		})
	}
	return responses, nil
}

func publicMinute(value time.Time) time.Time {
	return value.UTC().Round(time.Minute)
}

func publicMinutePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	rounded := publicMinute(*value)
	return &rounded
}

func publicStatusDisplay(status string) string {
	switch status {
	case "operational":
		return "Operational"
	case "degraded":
		return "Degraded"
	case "partial_outage":
		return "Partial outage"
	case "major_outage":
		return "Major outage"
	case "maintenance":
		return "Maintenance"
	default:
		return "Unknown"
	}
}
