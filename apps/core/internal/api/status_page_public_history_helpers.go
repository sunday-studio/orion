package api

import (
	"fmt"
	"math"
	"orion/core/internal/db"
	"time"
)

func publicUptimeBucket(date string, ratio *float64) StatusPagePublicUptimeBucketResponse {
	return StatusPagePublicUptimeBucketResponse{Date: date, Status: publicUptimeStatus(ratio), UptimeRatio: publicRoundedRatio(ratio), UptimeDisplay: publicUptimeDisplay(ratio)}
}
func publicUptimeStatus(ratio *float64) string {
	if ratio == nil {
		return "no_data"
	}
	if *ratio >= 0.99995 {
		return "operational"
	}
	if *ratio <= 0 {
		return "outage"
	}
	return "degraded"
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
	if err := s.db.Where("incident_id = ? AND published_at IS NOT NULL", incidentID).Order("published_at ASC").Find(&updates).Error; err != nil {
		return nil, err
	}
	responses := make([]StatusPagePublicIncidentUpdateHistoryResponse, 0, len(updates))
	for _, update := range updates {
		responses = append(responses, StatusPagePublicIncidentUpdateHistoryResponse{ID: update.ID, Status: update.Status, Message: update.Message, PublishedAt: publicMinutePtr(update.PublishedAt)})
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
