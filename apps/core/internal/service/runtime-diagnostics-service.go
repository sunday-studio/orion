package service

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"orion/core/internal/logging"

	"gorm.io/gorm"
)

const (
	diagnosticsSampleLimit         = 512
	ingestionSlowThreshold         = 500 * time.Millisecond
	reportWriteSlowThreshold       = 100 * time.Millisecond
	databaseOperationSlowThreshold = 100 * time.Millisecond
	databaseOperationStartKey      = "orion:runtime_diagnostics:database_operation_started_at"
	sqliteBusyErrorSubstring       = "database is locked"
	sqliteBusyExtendedSubstring    = "SQLITE_BUSY"
	sqliteBusyLowercaseSubstring   = "sqlite busy"
)

type RuntimeDiagnosticsService struct {
	db        *gorm.DB
	logger    *logging.Logger
	startedAt time.Time

	mu                            sync.Mutex
	requests                      map[string]map[string]int64
	requestDurations              map[string][]int64
	ingestionDurations            map[string][]int64
	reportWriteDurations          map[string][]int64
	reportWriteCounts             map[string]int64
	reportWriteErrors             map[string]int64
	incidentReconcileDurations    []int64
	incidentReconcileCount        int64
	incidentReconcileErrors       int64
	activeIncidentLookupDurations []int64
	activeIncidentLookupCount     int64
	activeIncidentLookupMisses    int64
	sqliteBusyTotal               int64
	slowOperations                []RuntimeSlowOperation
}

type RuntimeDiagnosticsSnapshot struct {
	Status                 string                               `json:"status"`
	UptimeSeconds          int64                                `json:"uptime_seconds"`
	StartedAt              time.Time                            `json:"started_at"`
	Requests               map[string]map[string]int64          `json:"requests"`
	RequestLatency         map[string]RuntimeLatencyPercentiles `json:"request_latency_ms"`
	Ingestion              map[string]RuntimeLatencyPercentiles `json:"ingestion_latency_ms"`
	ReportWrites           map[string]RuntimeReportWriteStats   `json:"report_writes"`
	IncidentReconciliation RuntimeOperationStats                `json:"incident_reconciliation"`
	ActiveIncidentLookup   RuntimeLookupStats                   `json:"active_incident_lookup"`
	SQLite                 RuntimeSQLiteStats                   `json:"sqlite"`
	SlowOperations         []RuntimeSlowOperation               `json:"slow_operations"`
}

type RuntimeLatencyPercentiles struct {
	Count int64 `json:"count"`
	P50   int64 `json:"p50"`
	P95   int64 `json:"p95"`
	P99   int64 `json:"p99"`
	Max   int64 `json:"max"`
}

type RuntimeReportWriteStats struct {
	Count      int64 `json:"count"`
	ErrorCount int64 `json:"error_count"`
	P50        int64 `json:"p50"`
	P95        int64 `json:"p95"`
	P99        int64 `json:"p99"`
	Max        int64 `json:"max"`
}

type RuntimeOperationStats struct {
	Count      int64 `json:"count"`
	ErrorCount int64 `json:"error_count"`
	P50        int64 `json:"p50"`
	P95        int64 `json:"p95"`
	P99        int64 `json:"p99"`
	Max        int64 `json:"max"`
}

type RuntimeLookupStats struct {
	Count     int64 `json:"count"`
	MissCount int64 `json:"miss_count"`
	P50       int64 `json:"p50"`
	P95       int64 `json:"p95"`
	P99       int64 `json:"p99"`
	Max       int64 `json:"max"`
}

type RuntimeSQLiteStats struct {
	BusyTotal     int64 `json:"busy_total"`
	DatabaseBytes int64 `json:"database_bytes"`
	PageCount     int64 `json:"page_count"`
	PageSizeBytes int64 `json:"page_size_bytes"`
	FreelistCount int64 `json:"freelist_count"`
}

type RuntimeSlowOperation struct {
	Kind       string    `json:"kind"`
	Operation  string    `json:"operation"`
	DurationMS int64     `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

func NewRuntimeDiagnosticsService(database *gorm.DB, logger *logging.Logger) *RuntimeDiagnosticsService {
	service := &RuntimeDiagnosticsService{
		db:                   database,
		logger:               logger,
		startedAt:            time.Now().UTC(),
		requests:             map[string]map[string]int64{},
		requestDurations:     map[string][]int64{},
		ingestionDurations:   map[string][]int64{},
		reportWriteDurations: map[string][]int64{},
		reportWriteCounts:    map[string]int64{},
		reportWriteErrors:    map[string]int64{},
	}
	service.registerGORMCallbacks()
	return service
}

func (s *RuntimeDiagnosticsService) RecordRequest(route string, status int, duration time.Duration) {
	if s == nil {
		return
	}
	statusKey := stringStatus(status)
	if route == "" {
		route = "unknown"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.requests[route]; !ok {
		s.requests[route] = map[string]int64{}
	}
	s.requests[route][statusKey]++
	s.requestDurations[route] = appendSample(s.requestDurations[route], duration.Milliseconds())
}

func (s *RuntimeDiagnosticsService) RecordIngestion(kind string, duration time.Duration, err error) {
	if s == nil {
		return
	}
	if kind == "" {
		kind = "unknown"
	}
	durationMS := duration.Milliseconds()

	s.mu.Lock()
	s.ingestionDurations[kind] = appendSample(s.ingestionDurations[kind], durationMS)
	if isSQLiteBusy(err) {
		s.sqliteBusyTotal++
	}
	s.mu.Unlock()

	if duration > ingestionSlowThreshold {
		s.RecordSlowOperation("ingestion", kind, duration)
		s.logger.Warn("Slow ingestion request", "kind", kind, "duration_ms", durationMS)
	}
}

func (s *RuntimeDiagnosticsService) RecordReportWrite(kind string, duration time.Duration, err error) {
	if s == nil {
		return
	}
	if kind == "" {
		kind = "unknown"
	}
	durationMS := duration.Milliseconds()

	s.mu.Lock()
	s.reportWriteCounts[kind]++
	s.reportWriteDurations[kind] = appendSample(s.reportWriteDurations[kind], durationMS)
	if err != nil {
		s.reportWriteErrors[kind]++
	}
	if isSQLiteBusy(err) {
		s.sqliteBusyTotal++
	}
	s.mu.Unlock()

	if duration > reportWriteSlowThreshold {
		s.RecordSlowOperation("report_write", kind, duration)
		s.logger.Warn("Slow report write", "kind", kind, "duration_ms", durationMS)
	}
}

func (s *RuntimeDiagnosticsService) RecordIncidentReconciliation(duration time.Duration, err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.incidentReconcileCount++
	s.incidentReconcileDurations = appendSample(s.incidentReconcileDurations, duration.Milliseconds())
	if err != nil {
		s.incidentReconcileErrors++
	}
	if isSQLiteBusy(err) {
		s.sqliteBusyTotal++
	}
	s.mu.Unlock()
}

func (s *RuntimeDiagnosticsService) RecordActiveIncidentLookup(duration time.Duration, found bool, err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.activeIncidentLookupCount++
	s.activeIncidentLookupDurations = appendSample(s.activeIncidentLookupDurations, duration.Milliseconds())
	if !found && err == nil {
		s.activeIncidentLookupMisses++
	}
	if isSQLiteBusy(err) {
		s.sqliteBusyTotal++
	}
	s.mu.Unlock()
}

func (s *RuntimeDiagnosticsService) RecordDatabaseOperation(operation string, duration time.Duration, err error) {
	if s == nil {
		return
	}
	if isSQLiteBusy(err) {
		s.mu.Lock()
		s.sqliteBusyTotal++
		s.mu.Unlock()
	}
	if duration > databaseOperationSlowThreshold {
		s.RecordSlowOperation("sqlite", operation, duration)
		if s.logger != nil {
			s.logger.Warn("Slow SQLite operation", "operation", operation, "duration_ms", duration.Milliseconds())
		}
	}
}

func (s *RuntimeDiagnosticsService) RecordSlowOperation(kind string, operation string, duration time.Duration) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.slowOperations = append(s.slowOperations, RuntimeSlowOperation{
		Kind:       kind,
		Operation:  operation,
		DurationMS: duration.Milliseconds(),
		CreatedAt:  time.Now().UTC(),
	})
	if len(s.slowOperations) > diagnosticsSampleLimit {
		s.slowOperations = s.slowOperations[len(s.slowOperations)-diagnosticsSampleLimit:]
	}
}

func (s *RuntimeDiagnosticsService) Snapshot(ctx context.Context) (RuntimeDiagnosticsSnapshot, error) {
	if s == nil {
		return RuntimeDiagnosticsSnapshot{}, nil
	}

	s.mu.Lock()
	snapshot := RuntimeDiagnosticsSnapshot{
		Status:                 "healthy",
		UptimeSeconds:          int64(time.Since(s.startedAt).Seconds()),
		StartedAt:              s.startedAt,
		Requests:               cloneNestedMap(s.requests),
		RequestLatency:         percentileMap(s.requestDurations),
		Ingestion:              percentileMap(s.ingestionDurations),
		ReportWrites:           s.reportWriteSnapshot(),
		IncidentReconciliation: operationStats(s.incidentReconcileCount, s.incidentReconcileErrors, s.incidentReconcileDurations),
		ActiveIncidentLookup:   lookupStats(s.activeIncidentLookupCount, s.activeIncidentLookupMisses, s.activeIncidentLookupDurations),
		SQLite: RuntimeSQLiteStats{
			BusyTotal: s.sqliteBusyTotal,
		},
		SlowOperations: append([]RuntimeSlowOperation(nil), s.slowOperations...),
	}
	s.mu.Unlock()

	sqliteStats, err := s.sqliteStats(ctx)
	if err != nil {
		return RuntimeDiagnosticsSnapshot{}, err
	}
	sqliteStats.BusyTotal = snapshot.SQLite.BusyTotal
	snapshot.SQLite = sqliteStats
	return snapshot, nil
}

func (s *RuntimeDiagnosticsService) reportWriteSnapshot() map[string]RuntimeReportWriteStats {
	result := make(map[string]RuntimeReportWriteStats, len(s.reportWriteCounts))
	for kind, count := range s.reportWriteCounts {
		result[kind] = reportWriteStats(count, s.reportWriteErrors[kind], s.reportWriteDurations[kind])
	}
	return result
}

func (s *RuntimeDiagnosticsService) registerGORMCallbacks() {
	if s == nil || s.db == nil {
		return
	}
	register := func(err error) {
		if err != nil && s.logger != nil {
			s.logger.Warn("Failed to register runtime diagnostics database callback", "error", err)
		}
	}

	register(s.db.Callback().Create().Before("gorm:create").Register("orion:runtime_diagnostics:create_start", s.beforeDatabaseOperation))
	register(s.db.Callback().Create().After("gorm:create").Register("orion:runtime_diagnostics:create_finish", s.afterDatabaseOperation("create")))
	register(s.db.Callback().Query().Before("gorm:query").Register("orion:runtime_diagnostics:query_start", s.beforeDatabaseOperation))
	register(s.db.Callback().Query().After("gorm:query").Register("orion:runtime_diagnostics:query_finish", s.afterDatabaseOperation("query")))
	register(s.db.Callback().Update().Before("gorm:update").Register("orion:runtime_diagnostics:update_start", s.beforeDatabaseOperation))
	register(s.db.Callback().Update().After("gorm:update").Register("orion:runtime_diagnostics:update_finish", s.afterDatabaseOperation("update")))
	register(s.db.Callback().Delete().Before("gorm:delete").Register("orion:runtime_diagnostics:delete_start", s.beforeDatabaseOperation))
	register(s.db.Callback().Delete().After("gorm:delete").Register("orion:runtime_diagnostics:delete_finish", s.afterDatabaseOperation("delete")))
	register(s.db.Callback().Raw().Before("gorm:raw").Register("orion:runtime_diagnostics:raw_start", s.beforeDatabaseOperation))
	register(s.db.Callback().Raw().After("gorm:raw").Register("orion:runtime_diagnostics:raw_finish", s.afterDatabaseOperation("raw")))
	register(s.db.Callback().Row().Before("gorm:row").Register("orion:runtime_diagnostics:row_start", s.beforeDatabaseOperation))
	register(s.db.Callback().Row().After("gorm:row").Register("orion:runtime_diagnostics:row_finish", s.afterDatabaseOperation("row")))
}

func (s *RuntimeDiagnosticsService) beforeDatabaseOperation(tx *gorm.DB) {
	tx.InstanceSet(databaseOperationStartKey, time.Now())
}

func (s *RuntimeDiagnosticsService) afterDatabaseOperation(operation string) func(*gorm.DB) {
	return func(tx *gorm.DB) {
		value, ok := tx.InstanceGet(databaseOperationStartKey)
		if !ok {
			return
		}
		startedAt, ok := value.(time.Time)
		if !ok {
			return
		}
		var err error
		if tx.Statement != nil {
			err = tx.Statement.Error
		}
		if err == nil {
			err = tx.Error
		}
		s.RecordDatabaseOperation(operation, time.Since(startedAt), err)
	}
}

func reportWriteStats(count int64, errorCount int64, samples []int64) RuntimeReportWriteStats {
	latency := percentiles(samples)
	return RuntimeReportWriteStats{
		Count:      count,
		ErrorCount: errorCount,
		P50:        latency.P50,
		P95:        latency.P95,
		P99:        latency.P99,
		Max:        latency.Max,
	}
}

func operationStats(count int64, errorCount int64, samples []int64) RuntimeOperationStats {
	latency := percentiles(samples)
	return RuntimeOperationStats{
		Count:      count,
		ErrorCount: errorCount,
		P50:        latency.P50,
		P95:        latency.P95,
		P99:        latency.P99,
		Max:        latency.Max,
	}
}

func lookupStats(count int64, missCount int64, samples []int64) RuntimeLookupStats {
	latency := percentiles(samples)
	return RuntimeLookupStats{
		Count:     count,
		MissCount: missCount,
		P50:       latency.P50,
		P95:       latency.P95,
		P99:       latency.P99,
		Max:       latency.Max,
	}
}

func (s *RuntimeDiagnosticsService) sqliteStats(ctx context.Context) (RuntimeSQLiteStats, error) {
	var pageCount int64
	if err := s.db.WithContext(ctx).Raw("PRAGMA page_count").Scan(&pageCount).Error; err != nil {
		return RuntimeSQLiteStats{}, err
	}
	var pageSize int64
	if err := s.db.WithContext(ctx).Raw("PRAGMA page_size").Scan(&pageSize).Error; err != nil {
		return RuntimeSQLiteStats{}, err
	}
	var freelistCount int64
	if err := s.db.WithContext(ctx).Raw("PRAGMA freelist_count").Scan(&freelistCount).Error; err != nil {
		return RuntimeSQLiteStats{}, err
	}
	return RuntimeSQLiteStats{
		DatabaseBytes: pageCount * pageSize,
		PageCount:     pageCount,
		PageSizeBytes: pageSize,
		FreelistCount: freelistCount,
	}, nil
}

func appendSample(samples []int64, value int64) []int64 {
	if value < 0 {
		value = 0
	}
	samples = append(samples, value)
	if len(samples) > diagnosticsSampleLimit {
		return samples[len(samples)-diagnosticsSampleLimit:]
	}
	return samples
}

func percentiles(samples []int64) RuntimeLatencyPercentiles {
	if len(samples) == 0 {
		return RuntimeLatencyPercentiles{}
	}
	sorted := append([]int64(nil), samples...)
	slices.Sort(sorted)
	return RuntimeLatencyPercentiles{
		Count: int64(len(samples)),
		P50:   percentile(sorted, 50),
		P95:   percentile(sorted, 95),
		P99:   percentile(sorted, 99),
		Max:   sorted[len(sorted)-1],
	}
}

func percentile(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	index := (len(sorted)*p + 99) / 100
	if index <= 0 {
		index = 1
	}
	if index > len(sorted) {
		index = len(sorted)
	}
	return sorted[index-1]
}

func percentileMap(values map[string][]int64) map[string]RuntimeLatencyPercentiles {
	result := make(map[string]RuntimeLatencyPercentiles, len(values))
	for key, samples := range values {
		result[key] = percentiles(samples)
	}
	return result
}

func cloneNestedMap(values map[string]map[string]int64) map[string]map[string]int64 {
	result := make(map[string]map[string]int64, len(values))
	for key, nested := range values {
		result[key] = make(map[string]int64, len(nested))
		for nestedKey, value := range nested {
			result[key][nestedKey] = value
		}
	}
	return result
}

func stringStatus(status int) string {
	if status <= 0 {
		return "unknown"
	}
	return strconv.Itoa(status)
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	lower := strings.ToLower(message)
	return strings.Contains(message, sqliteBusyExtendedSubstring) ||
		strings.Contains(lower, sqliteBusyErrorSubstring) ||
		strings.Contains(lower, sqliteBusyLowercaseSubstring)
}
