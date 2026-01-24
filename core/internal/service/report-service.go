package service

import (
	"encoding/json"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type AgentReportPayload struct {
	UptimeSeconds uint64         `json:"uptime_seconds"`
	Timestamp     string         `json:"timestamp"`
	CPU           db.CPUStats    `json:"cpu"`
	Memory        db.MemoryStats `json:"memory"`
	Disk          db.DiskStats   `json:"disk"`
	Location      db.GeoLocation `json:"location,omitempty"`
}

type MonitorReportPayload struct {
	Timestamp string      `json:"timestamp" binding:"required"`
	Health    string      `json:"health" binding:"required"` // up | down
	Metrics   interface{} `json:"metrics" binding:"required"`
	Error     interface{} `json:"error,omitempty"`
}

type ReportService struct {
	db     *gorm.DB
	logger *logging.Logger
}

func NewReportService(database *gorm.DB, logger *logging.Logger) *ReportService {
	return &ReportService{
		db:     database,
		logger: logger,
	}
}

func (s *ReportService) StoreMonitorReport(monitorID string, payload MonitorReportPayload) (*string, error) {
	monitorReportID := utils.GenerateID("monitor_report")

	// if health is down, store the error as payload
	var payloadData string

	// if payload.Health == "down" {
	// 	payloadJSON, err := json.Marshal(payload)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	payloadData = string(payloadJSON)
	// }

	if payload.Error != nil {
		payloadJSON, err := json.Marshal(payload.Error)
		if err != nil {
			return nil, err
		}
		payloadData = string(payloadJSON)
	}

	payloadJSON, err := json.Marshal(payload.Metrics)
	if err != nil {
		return nil, err
	}
	payloadData = string(payloadJSON)

	monitorReport := db.MonitorReport{
		ID:          monitorReportID,
		MonitorID:   monitorID,
		Health:      payload.Health,
		CollectedAt: payload.Timestamp,
		Payload:     payloadData,
	}

	if err := s.db.Create(&monitorReport).Error; err != nil {
		s.logger.Error("Failed to store monitor report", err)
		return nil, err
	}

	// Update monitor health and last successful report timestamp
	now := time.Now()
	updates := map[string]interface{}{
		"health": payload.Health,
	}

	// Only update last successful report if health is "up"
	if payload.Health == "up" {
		updates["last_successful_report_at"] = &now
	}

	if err := s.db.Model(&db.Monitor{}).Where("id = ?", monitorID).Updates(updates).Error; err != nil {
		s.logger.Error("Failed to update monitor health", "monitor_id", monitorID, "error", err)
		// Don't fail the request if monitor update fails
	}

	// Trigger health computation to update cache (async, don't block report storage)
	// This ensures cache is refreshed when new reports arrive
	go func() {
		healthService := NewHealthService(s.db, s.logger)
		config := DefaultHealthConfig()
		_, err := healthService.ComputeMonitorHealth(monitorID, config)
		if err != nil {
			s.logger.Error("Failed to compute health after report", "monitor_id", monitorID, "error", err)
		} else {
			s.logger.Debug("Health cache updated after report", "monitor_id", monitorID)
		}
	}()

	s.logger.Info("Monitor report stored successfully", "monitor_report_id ->", monitorReport.ID)
	return &monitorReportID, nil
}

func (s *ReportService) StoreAgentReport(agentID string, payload AgentReportPayload) (*string, error) {
	agentReportID := utils.GenerateID("agent_report")

	agentReport := db.AgentReport{
		ID:            agentReportID,
		AgentID:       agentID,
		UptimeSeconds: payload.UptimeSeconds,
		Timestamp:     payload.Timestamp,

		CPU:      datatypes.NewJSONType(payload.CPU),
		Memory:   datatypes.NewJSONType(payload.Memory),
		Disk:     datatypes.NewJSONType(payload.Disk),
		Location: datatypes.NewJSONType(payload.Location),
	}

	if err := s.db.Create(&agentReport).Omit("Agent").Error; err != nil {
		s.logger.Error("Failed to store agent report", err)
		return nil, err
	}

	s.logger.Info("Agent report stored successfully", "agent_report_id ->", agentReport.ID)
	return &agentReportID, nil
}

func (s *ReportService) GetAgentReportsById(agentID string, limit int, offset int) ([]db.AgentReport, error) {
	var reports []db.AgentReport

	query := s.db.Where("agent_id = ?", agentID).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&reports).Error; err != nil {
		s.logger.Error("Failed to retrieve reports", "agent_id", agentID, "error", err)
		return nil, err
	}

	s.logger.Debug("Retrieved reports", "agent_id", agentID, "count", len(reports))
	return reports, nil
}

func (s *ReportService) GetAgentReportCountById(agentID string) (int64, error) {
	var count int64

	if err := s.db.Model(&db.AgentReport{}).Where("agent_id = ?", agentID).Count(&count).Error; err != nil {
		s.logger.Error("Failed to count reports", "agent_id", agentID, "error", err)
		return 0, err
	}

	return count, nil
}

func (s *ReportService) GetMonitorReports(monitorID string, limit int, offset int) ([]db.MonitorReport, error) {
	var reports []db.MonitorReport

	query := s.db.Where("monitor_id = ?", monitorID).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&reports).Error; err != nil {
		s.logger.Error("Failed to retrieve monitor reports", "monitor_id", monitorID, "error", err)
		return nil, err
	}

	s.logger.Debug("Retrieved monitor reports", "monitor_id", monitorID, "count", len(reports))
	return reports, nil
}

func (s *ReportService) GetMonitorReportCount(monitorID string) (int64, error) {
	var count int64
	if err := s.db.Model(&db.MonitorReport{}).Where("monitor_id = ?", monitorID).Count(&count).Error; err != nil {
		s.logger.Error("Failed to count monitor reports", "monitor_id", monitorID, "error", err)
		return 0, err
	}
	return count, nil
}

// UptimeDayBucket is one day in the SLA heatmap.
type UptimeDayBucket struct {
	Date          string  `json:"date"` // YYYY-MM-DD
	Up            int     `json:"up"`
	Total         int     `json:"total"`
	UptimePercent float64 `json:"uptime_percent"`
}

// UptimeResult holds daily_buckets and overall uptime_percent.
type UptimeResult struct {
	DailyBuckets  []UptimeDayBucket `json:"daily_buckets"`
	UptimePercent float64           `json:"uptime_percent"`
}

// parsePeriod returns days from strings like "90d", "30d", "7d". Default 90.
func parsePeriod(period string) int {
	if period == "" {
		return 90
	}
	var n int
	for _, r := range period {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
		} else if r == 'd' && n > 0 {
			return n
		}
	}
	if n > 0 {
		return n
	}
	return 90
}

// GetMonitorUptime returns daily buckets and overall uptime percent for a monitor.
func (s *ReportService) GetMonitorUptime(monitorID string, period string) (*UptimeResult, error) {
	days := parsePeriod(period)
	since := time.Now().AddDate(0, 0, -days)

	var reports []db.MonitorReport
	if err := s.db.Where("monitor_id = ? AND created_at >= ?", monitorID, since).
		Order("created_at ASC").
		Find(&reports).Error; err != nil {
		s.logger.Error("Failed to get monitor reports for uptime", "monitor_id", monitorID, "error", err)
		return nil, err
	}

	// Bucket by date (day)
	type day struct {
		up, total int
	}
	buckets := make(map[string]*day)
	var totalUp, totalCount int

	for _, r := range reports {
		dateKey := r.CreatedAt.Format("2006-01-02")
		if buckets[dateKey] == nil {
			buckets[dateKey] = &day{}
		}
		buckets[dateKey].total++
		totalCount++
		if r.Health == "up" {
			buckets[dateKey].up++
			totalUp++
		}
	}

	// Build ordered daily_buckets for each day in range
	var daily []UptimeDayBucket
	for d := 0; d < days; d++ {
		dt := since.AddDate(0, 0, d)
		dateKey := dt.Format("2006-01-02")
		b := buckets[dateKey]
		up, total := 0, 0
		var pct float64
		if b != nil {
			up, total = b.up, b.total
			if total > 0 {
				pct = 100 * float64(up) / float64(total)
			}
		}
		daily = append(daily, UptimeDayBucket{Date: dateKey, Up: up, Total: total, UptimePercent: pct})
	}

	pct := 0.0
	if totalCount > 0 {
		pct = 100 * float64(totalUp) / float64(totalCount)
	}

	return &UptimeResult{DailyBuckets: daily, UptimePercent: pct}, nil
}

// GetAgentUptime aggregates uptime from the agent's monitors (average of each monitor's uptime_percent).
func (s *ReportService) GetAgentUptime(agentID string, period string) (*UptimeResult, error) {
	var monitorIDs []string
	if err := s.db.Model(&db.Monitor{}).Where("agent_id = ? AND lifecycle = ?", agentID, "active").Pluck("id", &monitorIDs).Error; err != nil {
		s.logger.Error("Failed to list monitors for agent uptime", "agent_id", agentID, "error", err)
		return nil, err
	}

	if len(monitorIDs) == 0 {
		days := parsePeriod(period)
		var daily []UptimeDayBucket
		for d := 0; d < days; d++ {
			dt := time.Now().AddDate(0, 0, -days+d)
			daily = append(daily, UptimeDayBucket{Date: dt.Format("2006-01-02"), Up: 0, Total: 0, UptimePercent: 0})
		}
		return &UptimeResult{DailyBuckets: daily, UptimePercent: 0}, nil
	}

	var sumPct float64
	var first *UptimeResult
	for _, mid := range monitorIDs {
		res, err := s.GetMonitorUptime(mid, period)
		if err != nil {
			continue
		}
		if first == nil {
			first = res
		}
		sumPct += res.UptimePercent
	}

	avg := 0.0
	if len(monitorIDs) > 0 {
		avg = sumPct / float64(len(monitorIDs))
	}

	// Use first monitor's daily_buckets as shape; we could average each day across monitors but that's heavier.
	// For agent we aggregate by average of overall uptime; daily_buckets can mirror one monitor or be simplified.
	if first == nil {
		days := parsePeriod(period)
		var daily []UptimeDayBucket
		for d := 0; d < days; d++ {
			dt := time.Now().AddDate(0, 0, -days+d)
			daily = append(daily, UptimeDayBucket{Date: dt.Format("2006-01-02"), Up: 0, Total: 0, UptimePercent: 0})
		}
		return &UptimeResult{DailyBuckets: daily, UptimePercent: avg}, nil
	}

	return &UptimeResult{DailyBuckets: first.DailyBuckets, UptimePercent: avg}, nil
}
