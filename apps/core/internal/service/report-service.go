package service

import (
	"encoding/json"
	"orion/core/internal/config"
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
	AgentVersion  string         `json:"agent_version,omitempty"`
	ConfigSummary interface{}    `json:"config_summary,omitempty"`
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
	db          *gorm.DB
	logger      *logging.Logger
	cfg         *config.Config
	diagnostics *RuntimeDiagnosticsService
}

func NewReportService(database *gorm.DB, logger *logging.Logger, cfg *config.Config) *ReportService {
	return &ReportService{
		db:     database,
		logger: logger,
		cfg:    cfg,
	}
}

func (s *ReportService) SetDiagnostics(diagnostics *RuntimeDiagnosticsService) {
	s.diagnostics = diagnostics
}

func (s *ReportService) StoreMonitorReport(monitorID string, payload MonitorReportPayload) (*string, error) {
	monitorReportID := utils.GenerateID("monitor_report")

	var payloadData string
	if payload.Error != nil {
		b, err := json.Marshal(payload.Error)
		if err != nil {
			return nil, err
		}
		payloadData = string(b)
	} else {
		b, err := json.Marshal(payload.Metrics)
		if err != nil {
			return nil, err
		}
		payloadData = string(b)
	}

	monitorReport := db.MonitorReport{
		ID:          monitorReportID,
		MonitorID:   monitorID,
		Health:      payload.Health,
		CollectedAt: payload.Timestamp,
		Payload:     payloadData,
	}

	writeStarted := time.Now()
	if err := s.db.Create(&monitorReport).Error; err != nil {
		s.diagnostics.RecordReportWrite("monitor", time.Since(writeStarted), err)
		s.logger.Error("Failed to store monitor report", "error", err)
		return nil, err
	}
	s.diagnostics.RecordReportWrite("monitor", time.Since(writeStarted), nil)

	// Update monitor health and last successful report timestamp
	now := time.Now()
	updates := map[string]interface{}{
		"health":                  payload.Health,
		"computed_health":         "",
		"last_health_computation": nil,
	}

	// Only update last successful report if health is "up"
	if payload.Health == "up" {
		updates["last_successful_report_at"] = &now
	}

	if err := s.db.Model(&db.Monitor{}).Where("id = ?", monitorID).Updates(updates).Error; err != nil {
		s.logger.Error("Failed to update monitor health", "monitor_id", monitorID, "error", err)
		// Don't fail the request if monitor update fails
	}

	incidentService := NewIncidentService(s.db, s.logger, s.cfg)
	incidentService.SetDiagnostics(s.diagnostics)
	if err := incidentService.ReconcileMonitorReport(monitorID, monitorReportID, payload); err != nil {
		s.logger.Error("Failed to reconcile incident after monitor report", "monitor_id", monitorID, "monitor_report_id", monitorReportID, "error", err)
		return nil, err
	}

	healthService := NewHealthService(s.db, s.logger)
	config := DefaultHealthConfig()
	if _, err := healthService.ComputeMonitorHealth(monitorID, config); err != nil {
		s.logger.Error("Failed to compute health after report", "monitor_id", monitorID, "error", err)
	}

	s.logger.Info("Monitor report stored successfully", "monitor_report_id", monitorReport.ID)
	return &monitorReportID, nil
}

func (s *ReportService) StoreAgentReport(agentID string, payload AgentReportPayload) (*string, error) {
	agentReportID := utils.GenerateID("agent_report")
	configSummary := ""
	if payload.ConfigSummary != nil {
		configSummaryBytes, err := json.Marshal(payload.ConfigSummary)
		if err != nil {
			return nil, err
		}
		configSummary = string(configSummaryBytes)
	}

	agentReport := db.AgentReport{
		ID:            agentReportID,
		AgentID:       agentID,
		AgentVersion:  payload.AgentVersion,
		ConfigSummary: configSummary,
		UptimeSeconds: payload.UptimeSeconds,
		Timestamp:     payload.Timestamp,

		CPU:      datatypes.NewJSONType(payload.CPU),
		Memory:   datatypes.NewJSONType(payload.Memory),
		Disk:     datatypes.NewJSONType(payload.Disk),
		Location: datatypes.NewJSONType(payload.Location),
	}

	writeStarted := time.Now()
	if err := s.db.Create(&agentReport).Omit("Agent").Error; err != nil {
		s.diagnostics.RecordReportWrite("agent", time.Since(writeStarted), err)
		s.logger.Error("Failed to store agent report", "error", err)
		return nil, err
	}
	s.diagnostics.RecordReportWrite("agent", time.Since(writeStarted), nil)

	if intervalSeconds := reportingIntervalSeconds(payload.ConfigSummary); intervalSeconds > 0 {
		if err := s.db.Model(&db.Agent{}).
			Where("id = ?", agentID).
			Update("reporting_interval_seconds", intervalSeconds).Error; err != nil {
			s.logger.Error("Failed to update agent reporting interval", "agent_id", agentID, "error", err)
		}
	}

	s.logger.Info("Agent report stored successfully", "agent_report_id", agentReport.ID)
	incidentService := NewIncidentService(s.db, s.logger, s.cfg)
	incidentService.SetDiagnostics(s.diagnostics)
	if err := incidentService.ReconcileStaleMonitors(agentID); err != nil {
		s.logger.Error("Failed to reconcile stale monitor incidents after agent report", "agent_id", agentID, "error", err)
		return nil, err
	}
	return &agentReportID, nil
}

func reportingIntervalSeconds(configSummary interface{}) int {
	summary, ok := configSummary.(map[string]interface{})
	if !ok {
		return 0
	}

	value, ok := summary["reporting_interval"]
	if !ok {
		return 0
	}

	interval, ok := value.(string)
	if !ok || interval == "" {
		return 0
	}

	duration, err := time.ParseDuration(interval)
	if err != nil || duration <= 0 {
		return 0
	}

	return int(duration.Seconds())
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
	UpCount       int               `json:"-"`
	TotalCount    int               `json:"-"`
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
	return s.getMonitorUptime(monitorID, period, time.Now())
}

func (s *ReportService) getMonitorUptime(monitorID string, period string, now time.Time) (*UptimeResult, error) {
	days := parsePeriod(period)
	since := now.AddDate(0, 0, -days)
	settingsService := NewSettingsService(s.db, s.logger, s.cfg.DataDir)
	settings, err := settingsService.GetDataLifecycleSettings()
	if err != nil {
		return nil, err
	}
	rawCutoffDay := dayStart(now.AddDate(0, 0, -settings.RawReportHotDays))

	var reports []db.MonitorReport
	if err := s.db.Where("monitor_id = ? AND created_at >= ? AND created_at >= ?", monitorID, since, rawCutoffDay).
		Order("created_at ASC").
		Find(&reports).Error; err != nil {
		s.logger.Error("Failed to get monitor reports for uptime", "monitor_id", monitorID, "error", err)
		return nil, err
	}

	var rollups []db.MonitorUptimeRollup
	if err := s.db.Where("monitor_id = ? AND date >= ? AND date < ?", monitorID, since.Format("2006-01-02"), rawCutoffDay.Format("2006-01-02")).
		Order("date ASC").
		Find(&rollups).Error; err != nil {
		s.logger.Error("Failed to get monitor uptime rollups", "monitor_id", monitorID, "error", err)
		return nil, err
	}

	// Bucket by date (day)
	type day struct {
		up, total int
	}
	buckets := make(map[string]*day)
	var totalUp, totalCount int

	for _, r := range rollups {
		buckets[r.Date] = &day{up: r.UpCount, total: r.TotalCount}
		totalUp += r.UpCount
		totalCount += r.TotalCount
	}

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

	return &UptimeResult{DailyBuckets: daily, UptimePercent: pct, UpCount: totalUp, TotalCount: totalCount}, nil
}

// GetAgentUptime aggregates uptime from the agent's active monitors.
func (s *ReportService) GetAgentUptime(agentID string, period string) (*UptimeResult, error) {
	return s.getAgentUptime(agentID, period, time.Now())
}

func (s *ReportService) getAgentUptime(agentID string, period string, now time.Time) (*UptimeResult, error) {
	var monitorIDs []string
	if err := s.db.Model(&db.Monitor{}).Where("agent_id = ? AND lifecycle = ?", agentID, "active").Pluck("id", &monitorIDs).Error; err != nil {
		s.logger.Error("Failed to list monitors for agent uptime", "agent_id", agentID, "error", err)
		return nil, err
	}

	if len(monitorIDs) == 0 {
		return &UptimeResult{DailyBuckets: emptyUptimeBuckets(period, now), UptimePercent: 0}, nil
	}

	dailyByDate := make(map[string]*UptimeDayBucket)
	var totalUp, totalCount int
	for _, mid := range monitorIDs {
		res, err := s.getMonitorUptime(mid, period, now)
		if err != nil {
			continue
		}

		for _, bucket := range res.DailyBuckets {
			agentBucket := dailyByDate[bucket.Date]
			if agentBucket == nil {
				agentBucket = &UptimeDayBucket{Date: bucket.Date}
				dailyByDate[bucket.Date] = agentBucket
			}
			agentBucket.Up += bucket.Up
			agentBucket.Total += bucket.Total
			totalUp += bucket.Up
			totalCount += bucket.Total
		}
	}

	days := parsePeriod(period)
	since := now.AddDate(0, 0, -days)
	daily := make([]UptimeDayBucket, 0, days)
	for d := 0; d < days; d++ {
		dateKey := since.AddDate(0, 0, d).Format("2006-01-02")
		bucket := UptimeDayBucket{Date: dateKey}
		if aggregated := dailyByDate[dateKey]; aggregated != nil {
			bucket.Up = aggregated.Up
			bucket.Total = aggregated.Total
			if bucket.Total > 0 {
				bucket.UptimePercent = 100 * float64(bucket.Up) / float64(bucket.Total)
			}
		}
		daily = append(daily, bucket)
	}

	pct := 0.0
	if totalCount > 0 {
		pct = 100 * float64(totalUp) / float64(totalCount)
	}

	return &UptimeResult{DailyBuckets: daily, UptimePercent: pct, UpCount: totalUp, TotalCount: totalCount}, nil
}

func emptyUptimeBuckets(period string, now time.Time) []UptimeDayBucket {
	days := parsePeriod(period)
	since := now.AddDate(0, 0, -days)
	daily := make([]UptimeDayBucket, 0, days)
	for d := 0; d < days; d++ {
		dt := since.AddDate(0, 0, d)
		daily = append(daily, UptimeDayBucket{Date: dt.Format("2006-01-02")})
	}
	return daily
}
