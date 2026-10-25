package service

import (
	"encoding/json"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NodeReportPayload struct {
	UptimeSeconds uint64         `json:"uptime_seconds"`
	Timestamp     string         `json:"timestamp"`
	NodeVersion   string         `json:"node_version,omitempty"`
	ConfigSummary any            `json:"config_summary,omitempty"`
	CPU           db.CPUStats    `json:"cpu"`
	Memory        db.MemoryStats `json:"memory"`
	Disk          db.DiskStats   `json:"disk"`
	Location      db.GeoLocation `json:"location,omitempty"`
	// AppResources is the node's passive per-app measurements; Core stores the latest.
	AppResources json.RawMessage `json:"app_resources,omitempty" swaggertype:"object"`
}

type MonitorReportPayload struct {
	// ReportID is an optional node-generated idempotency key (UUID). Replaying a
	// report with the same ID for the same monitor stores nothing new.
	ReportID  string `json:"report_id,omitempty"`
	Timestamp string `json:"timestamp" binding:"required"`
	Health    string `json:"health" binding:"required"` // up | down
	Metrics   any    `json:"metrics" binding:"required"`
	Error     any    `json:"error,omitempty"`
}

type ReportService struct {
	db          *gorm.DB
	logger      *utils.Logger
	cfg         *config.Config
	diagnostics *RuntimeDiagnosticsService
}

func NewReportService(database *gorm.DB, logger *utils.Logger, cfg *config.Config) *ReportService {
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

	var reportPayload interface{}
	if payload.Error != nil {
		reportPayload = payload.Error
	} else {
		reportPayload = payload.Metrics
	}

	b, err := json.Marshal(RedactMonitorReportPayloadValue(reportPayload))
	if err != nil {
		return nil, err
	}

	monitorReport := db.MonitorReport{
		ID:             monitorReportID,
		MonitorID:      monitorID,
		ClientReportID: strings.TrimSpace(payload.ReportID),
		Health:         payload.Health,
		CollectedAt:    payload.Timestamp,
		Payload:        string(b),
	}

	// The report row, the monitor's health columns and the incident reconcile
	// commit together. A failure rolls everything back, so the node's retry
	// starts clean instead of finding a stored report whose incident was never
	// opened; a replay of an already-committed report is a no-op.
	storedID := monitorReportID
	duplicate := false
	writeStarted := time.Now()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&monitorReport)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var existing db.MonitorReport
			if err := tx.Where("monitor_id = ? AND client_report_id = ?", monitorID, monitorReport.ClientReportID).First(&existing).Error; err != nil {
				return err
			}
			storedID = existing.ID
			duplicate = true
			return nil
		}
		s.diagnostics.RecordReportWrite("monitor", time.Since(writeStarted), nil)

		now := time.Now()
		updates := map[string]any{
			"health":                  payload.Health,
			"computed_health":         "",
			"last_health_computation": nil,
		}
		if payload.Health == "up" {
			updates["last_successful_report_at"] = &now
		}
		if err := tx.Model(&db.Monitor{}).Where("id = ?", monitorID).Updates(updates).Error; err != nil {
			return err
		}

		incidentService := NewIncidentService(tx, s.logger, s.cfg)
		incidentService.SetDiagnostics(s.diagnostics)
		incidentService.inTransaction = true
		return incidentService.ReconcileMonitorReport(monitorID, monitorReportID, payload)
	})
	if err != nil {
		s.diagnostics.RecordReportWrite("monitor", time.Since(writeStarted), err)
		s.logger.Error("Failed to store monitor report", "monitor_id", monitorID, "error", err)
		return nil, err
	}
	if duplicate {
		s.logger.Info("Duplicate monitor report ignored", "monitor_id", monitorID, "monitor_report_id", storedID)
		return &storedID, nil
	}

	healthService := NewHealthService(s.db, s.logger)
	if _, err := healthService.ComputeMonitorHealth(monitorID, DefaultHealthConfig()); err != nil {
		s.logger.Error("Failed to compute health after report", "monitor_id", monitorID, "error", err)
	}

	s.logger.Info("Monitor report stored successfully", "monitor_report_id", storedID)
	return &storedID, nil
}

func (s *ReportService) StoreNodeReport(nodeID string, payload NodeReportPayload) (*string, error) {
	nodeReportID := utils.GenerateID("node_report")
	configSummary := ""
	if payload.ConfigSummary != nil {
		configSummaryBytes, err := json.Marshal(payload.ConfigSummary)
		if err != nil {
			return nil, err
		}
		configSummary = string(configSummaryBytes)
	}

	nodeReport := db.NodeReport{
		ID:            nodeReportID,
		NodeID:        nodeID,
		NodeVersion:   payload.NodeVersion,
		ConfigSummary: configSummary,
		UptimeSeconds: payload.UptimeSeconds,
		Timestamp:     payload.Timestamp,

		CPU:      datatypes.NewJSONType(payload.CPU),
		Memory:   datatypes.NewJSONType(payload.Memory),
		Disk:     datatypes.NewJSONType(payload.Disk),
		Location: datatypes.NewJSONType(payload.Location),
	}

	writeStarted := time.Now()
	if err := s.db.Create(&nodeReport).Omit("Node").Error; err != nil {
		s.diagnostics.RecordReportWrite("node", time.Since(writeStarted), err)
		s.logger.Error("Failed to store node report", "error", err)
		return nil, err
	}
	s.diagnostics.RecordReportWrite("node", time.Since(writeStarted), nil)

	if intervalSeconds := reportingIntervalSeconds(payload.ConfigSummary); intervalSeconds > 0 {
		if err := s.db.Model(&db.Node{}).
			Where("id = ?", nodeID).
			Update("reporting_interval_seconds", intervalSeconds).Error; err != nil {
			s.logger.Error("Failed to update node reporting interval", "node_id", nodeID, "error", err)
		}
	}

	s.logger.Info("Node report stored successfully", "node_report_id", nodeReport.ID)
	incidentService := NewIncidentService(s.db, s.logger, s.cfg)
	incidentService.SetDiagnostics(s.diagnostics)
	if err := incidentService.ReconcileStaleMonitors(nodeID); err != nil {
		s.logger.Error("Failed to reconcile stale monitor incidents after node report", "node_id", nodeID, "error", err)
		return nil, err
	}
	return &nodeReportID, nil
}

func reportingIntervalSeconds(configSummary any) int {
	summary, ok := configSummary.(map[string]any)
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

func (s *ReportService) GetNodeReportsById(nodeID string, limit int, offset int) ([]db.NodeReport, error) {
	var reports []db.NodeReport

	query := s.db.Where("node_id = ?", nodeID).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&reports).Error; err != nil {
		s.logger.Error("Failed to retrieve reports", "node_id", nodeID, "error", err)
		return nil, err
	}

	s.logger.Debug("Retrieved reports", "node_id", nodeID, "count", len(reports))
	return reports, nil
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

const (
	defaultPeriodDays = 90
	maxPeriodDays     = 365
)

// parsePeriod returns days from strings like "90d", "30d", "7d". Default 90,
// clamped to maxPeriodDays so a hostile period cannot allocate unbounded buckets.
func parsePeriod(period string) int {
	if period == "" {
		return defaultPeriodDays
	}
	var n int
	for _, r := range period {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
			if n > maxPeriodDays {
				return maxPeriodDays
			}
		} else if r == 'd' && n > 0 {
			return n
		}
	}
	if n > 0 {
		return n
	}
	return defaultPeriodDays
}

// GetMonitorUptime returns daily buckets and overall uptime percent for a monitor.
func (s *ReportService) GetMonitorUptime(monitorID string, period string) (*UptimeResult, error) {
	return s.getMonitorUptime(monitorID, period, time.Now().UTC())
}

const uptimeDateLayout = "2006-01-02"

// uptimeCounts is one monitor-day: checks that came back up, checks that arrived, checks that should
// have run but did not (downtime), and the monitor's interval in seconds as its weight when combined.
type uptimeCounts struct{ up, total, missed, weight int }

// parseStoredTime reads the text SQLite hands back for MIN/MAX over a timestamp column.
func parseStoredTime(s string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02 15:04:05.999999999-07:00", "2006-01-02T15:04:05.999999999Z07:00", "2006-01-02 15:04:05.999999999"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// monitorUptimeCounts returns per-monitor, per-day counts for the last `days` days.
//
// Completed days come from monitor_uptime_rollups, so the cost does not grow with report volume.
// Raw reports are only aggregated (in SQL, never loaded as rows) for today and for any completed
// day that has no rollup yet (for example yesterday before the nightly job, or a missed run). For
// those days silences before the first and after the last report count as missed checks; silences in
// the middle are added when the day is rolled up.
func (s *ReportService) monitorUptimeCounts(monitorIDs []string, days int, now time.Time) (map[string]map[string]*uptimeCounts, error) {
	out := make(map[string]map[string]*uptimeCounts, len(monitorIDs))
	for _, id := range monitorIDs {
		out[id] = make(map[string]*uptimeCounts)
	}
	if len(monitorIDs) == 0 {
		return out, nil
	}

	since := now.AddDate(0, 0, -days)
	sinceKey := since.Format(uptimeDateLayout)
	today := dayStart(now)
	todayKey := today.Format(uptimeDateLayout)

	monitors, err := loadUptimeMonitors(s.db, monitorIDs)
	if err != nil {
		return nil, err
	}
	weightOf := func(id string) int {
		if m, ok := monitors[id]; ok {
			return int(m.interval() / time.Second)
		}
		return 60
	}

	var rollups []db.MonitorUptimeRollup
	if err := s.db.Where("monitor_id IN ? AND date >= ? AND date < ?", monitorIDs, sinceKey, todayKey).Find(&rollups).Error; err != nil {
		s.logger.Error("Failed to get monitor uptime rollups", "error", err)
		return nil, err
	}
	hasRollup := make(map[string]map[string]bool, len(monitorIDs))
	for _, r := range rollups {
		weight := r.IntervalSeconds
		if weight <= 0 {
			weight = weightOf(r.MonitorID)
		}
		out[r.MonitorID][r.Date] = &uptimeCounts{up: r.UpCount, total: r.TotalCount, missed: r.MissedCount, weight: weight}
		if hasRollup[r.MonitorID] == nil {
			hasRollup[r.MonitorID] = make(map[string]bool)
		}
		hasRollup[r.MonitorID][r.Date] = true
	}

	// Completed days without a rollup still need raw data, but only from each monitor's first report
	// onward: days before it existed have nothing to count. One index seek per monitor, one round trip.
	firstReport := make(map[string]time.Time, len(monitorIDs))
	for from := 0; from < len(monitorIDs); from += 200 {
		chunk := monitorIDs[from:min(from+200, len(monitorIDs))]
		parts := make([]string, len(chunk))
		args := make([]any, 0, len(chunk)*2)
		for i, id := range chunk {
			parts[i] = "SELECT ? AS monitor_id, (SELECT MIN(created_at) FROM monitor_reports WHERE monitor_id = ?) AS first_at"
			args = append(args, id, id)
		}
		var firsts []struct {
			MonitorID string
			FirstAt   *string
		}
		if err := s.db.Raw(strings.Join(parts, " UNION ALL "), args...).Scan(&firsts).Error; err != nil {
			return nil, err
		}
		for _, f := range firsts {
			// MIN() loses the column's time type, so read the stored text and keep its date part.
			if f.FirstAt != nil && len(*f.FirstAt) >= 10 {
				if t, err := time.Parse(uptimeDateLayout, (*f.FirstAt)[:10]); err == nil {
					firstReport[f.MonitorID] = t
				}
			}
		}
	}
	missing := make(map[string]map[string]bool)
	rawFrom := today
	for _, id := range monitorIDs {
		firstAt, ok := firstReport[id]
		if !ok {
			continue
		}
		first := dayStart(since)
		if dayStart(firstAt).After(first) {
			first = dayStart(firstAt)
		}
		for d := first; d.Before(today); d = d.AddDate(0, 0, 1) {
			key := d.Format(uptimeDateLayout)
			if hasRollup[id][key] {
				continue
			}
			if missing[id] == nil {
				missing[id] = make(map[string]bool)
			}
			missing[id][key] = true
			if d.Before(rawFrom) {
				rawFrom = d
			}
		}
	}

	var rows []struct {
		MonitorID string
		Day       string
		Up        int
		Total     int
		FirstAt   string
		LastAt    string
	}
	if err := s.db.Raw(`
		SELECT monitor_id, substr(created_at, 1, 10) AS day,
		       SUM(CASE WHEN health = 'up' THEN 1 ELSE 0 END) AS up,
		       COUNT(*) AS total,
		       MIN(created_at) AS first_at, MAX(created_at) AS last_at
		FROM monitor_reports
		WHERE monitor_id IN ? AND created_at >= ?
		GROUP BY monitor_id, day`, monitorIDs, rawFrom).Scan(&rows).Error; err != nil {
		s.logger.Error("Failed to aggregate monitor reports for uptime", "error", err)
		return nil, err
	}

	// Gap accounting for the raw days: only from the day paused/maintenance history started.
	gapsFrom := gapAccountingStart(s.db)
	gapsOn := !gapsFrom.IsZero()
	var excluded map[string][]timeSpan
	if gapsOn {
		if excluded, err = loadExcludedSpans(s.db, timeSpan{rawFrom, now}, monitors); err != nil {
			return nil, err
		}
	}
	missedFor := func(id string, day time.Time, firstAt, lastAt *time.Time) int {
		m, ok := monitors[id]
		if !gapsOn || !ok || day.Before(gapsFrom) {
			return 0
		}
		window := timeSpan{day, day.AddDate(0, 0, 1)}
		if window.end.After(now) {
			window.end = now
		}
		cov := m.coverage(window)
		if !cov.end.After(cov.start) {
			return 0
		}
		if firstAt == nil {
			return silentMissedChecks(cov, m.interval(), excluded[id])
		}
		lead, trail := timeSpan{cov.start, *firstAt}, timeSpan{*lastAt, cov.end}
		return silentMissedChecks(lead, m.interval(), excluded[id]) + silentMissedChecks(trail, m.interval(), excluded[id])
	}

	seenRaw := make(map[string]map[string]bool)
	for _, r := range rows {
		if !(r.Day == todayKey || missing[r.MonitorID][r.Day]) {
			continue
		}
		day, err := time.Parse(uptimeDateLayout, r.Day)
		if err != nil {
			continue
		}
		first, okFirst := parseStoredTime(r.FirstAt)
		last, okLast := parseStoredTime(r.LastAt)
		counts := &uptimeCounts{up: r.Up, total: r.Total, weight: weightOf(r.MonitorID)}
		if okFirst && okLast {
			counts.missed = missedFor(r.MonitorID, day, &first, &last)
		}
		out[r.MonitorID][r.Day] = counts
		if seenRaw[r.MonitorID] == nil {
			seenRaw[r.MonitorID] = make(map[string]bool)
		}
		seenRaw[r.MonitorID][r.Day] = true
	}

	// Days (and today) with no report at all are silent for the whole time the monitor existed.
	if gapsOn {
		for id := range firstReport {
			dayKeys := []string{todayKey}
			for key := range missing[id] {
				dayKeys = append(dayKeys, key)
			}
			for _, key := range dayKeys {
				if seenRaw[id][key] {
					continue
				}
				day, err := time.Parse(uptimeDateLayout, key)
				if err != nil {
					continue
				}
				if missed := missedFor(id, day, nil, nil); missed > 0 {
					out[id][key] = &uptimeCounts{missed: missed, weight: weightOf(id)}
				}
			}
		}
	}
	return out, nil
}

// uptimeResultFromCounts builds the ordered daily buckets and overall percent for a period. Missed
// checks are downtime: a bucket's total is the checks that arrived plus the checks that did not.
func uptimeResultFromCounts(counts map[string]*uptimeCounts, days int, now time.Time) *UptimeResult {
	since := now.AddDate(0, 0, -days)
	daily := make([]UptimeDayBucket, 0, days)
	var totalUp, totalCount int
	for d := 0; d < days; d++ {
		key := since.AddDate(0, 0, d).Format(uptimeDateLayout)
		bucket := UptimeDayBucket{Date: key}
		if c := counts[key]; c != nil {
			bucket.Up, bucket.Total = c.up, c.total+c.missed
			if bucket.Total > 0 {
				bucket.UptimePercent = 100 * float64(c.up) / float64(bucket.Total)
			}
		}
		daily = append(daily, bucket)
	}
	// Totals cover every counted day, including today, which has no bucket of its own.
	for _, c := range counts {
		totalUp += c.up
		totalCount += c.total + c.missed
	}
	pct := 0.0
	if totalCount > 0 {
		pct = 100 * float64(totalUp) / float64(totalCount)
	}
	return &UptimeResult{DailyBuckets: daily, UptimePercent: pct, UpCount: totalUp, TotalCount: totalCount}
}

func (s *ReportService) getMonitorUptime(monitorID string, period string, now time.Time) (*UptimeResult, error) {
	days := parsePeriod(period)
	counts, err := s.monitorUptimeCounts([]string{monitorID}, days, now)
	if err != nil {
		return nil, err
	}
	return uptimeResultFromCounts(counts[monitorID], days, now), nil
}

// GetNodeUptime aggregates uptime from the node's active monitors.
func (s *ReportService) GetNodeUptime(nodeID string, period string) (*UptimeResult, error) {
	return s.getNodeUptime(nodeID, period, time.Now().UTC())
}

// getNodeUptime combines the node's monitors weighted by time: each monitor's counts are scaled by its
// check interval, so a monitor checking every 10s counts the same as one checking every 60s. The
// returned UpCount and TotalCount are therefore seconds, not checks.
func (s *ReportService) getNodeUptime(nodeID string, period string, now time.Time) (*UptimeResult, error) {
	var monitorIDs []string
	if err := s.db.Model(&db.Monitor{}).Where("node_id = ? AND lifecycle = ?", nodeID, "active").Pluck("id", &monitorIDs).Error; err != nil {
		s.logger.Error("Failed to list monitors for node uptime", "node_id", nodeID, "error", err)
		return nil, err
	}

	if len(monitorIDs) == 0 {
		return &UptimeResult{DailyBuckets: emptyUptimeBuckets(period, now), UptimePercent: 0}, nil
	}

	days := parsePeriod(period)
	perMonitor, err := s.monitorUptimeCounts(monitorIDs, days, now)
	if err != nil {
		return nil, err
	}
	combined := make(map[string]*uptimeCounts)
	for _, byDay := range perMonitor {
		for day, c := range byDay {
			if combined[day] == nil {
				combined[day] = &uptimeCounts{}
			}
			w := c.weight
			if w <= 0 {
				w = 60
			}
			combined[day].up += c.up * w
			combined[day].total += c.total * w
			combined[day].missed += c.missed * w
		}
	}
	return uptimeResultFromCounts(combined, days, now), nil
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
