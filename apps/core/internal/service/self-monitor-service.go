package service

import (
	"context"
	"errors"
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"time"

	"gorm.io/gorm"
)

const (
	selfMonitorInterval    = 30 * time.Second
	deadManInterval        = time.Minute
	coreWorkerMonitorID    = "monitor-core-worker-liveness"
	coreWorkerMonitorName  = "Core worker"
	selfMonitorDispatchCap = 100
)

// SelfMonitorService lets Core watch itself from the API process, which is the
// one place guaranteed to be alive when the Worker is not:
//
//   - It reports the Worker's liveness as an ordinary monitor, so a dead or stuck
//     Worker opens an incident and pages through the normal alert pipeline.
//   - While the Worker is not online it also drains the alert queue itself
//     (deliveries are leased, so this can never double-send), otherwise the page
//     about the dead Worker would sit in a queue only the dead Worker empties.
//   - It pings ORION_DEADMAN_URL (for example a healthchecks.io check) every
//     minute; when Core itself dies the external service raises the alarm.
type SelfMonitorService struct {
	db         *gorm.DB
	logger     *utils.Logger
	cfg        *config.Config
	reports    *ReportService
	alerts     *AlertService
	httpClient *http.Client
}

func NewSelfMonitorService(database *gorm.DB, logger *utils.Logger, cfg *config.Config, diagnostics *RuntimeDiagnosticsService) *SelfMonitorService {
	reports := NewReportService(database, logger, cfg)
	reports.SetDiagnostics(diagnostics)
	return &SelfMonitorService{
		db:         database,
		logger:     logger,
		cfg:        cfg,
		reports:    reports,
		alerts:     NewAlertService(database, logger, cfg),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Run checks every 30s and sends the dead-man ping every minute until ctx ends.
func (s *SelfMonitorService) Run(ctx context.Context) {
	check := time.NewTicker(selfMonitorInterval)
	defer check.Stop()
	ping := time.NewTicker(deadManInterval)
	defer ping.Stop()

	s.pingDeadMan(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-check.C:
			if err := s.Check(ctx, now.UTC()); err != nil {
				s.logger.Error("Core self-monitor check failed", "error", err)
			}
		case <-ping.C:
			s.pingDeadMan(ctx)
		}
	}
}

// Check records the Worker's liveness and, if the Worker is down, delivers pending alerts itself.
func (s *SelfMonitorService) Check(ctx context.Context, now time.Time) error {
	staleAfter := time.Duration(s.cfg.CoreWorkerStaleSeconds) * time.Second
	diag, err := NewWorkerDiagnosticsService(s.db, s.logger).GetDiagnostics(ctx, staleAfter, now)
	if err != nil {
		return err
	}
	if diag.WorkerCount == 0 {
		return nil // no Worker has ever registered: it is optional, nothing to watch yet
	}

	healthy := diag.OnlineCount > 0
	if err := s.ensureWorkerMonitor(); err != nil {
		return err
	}
	health := "up"
	if !healthy {
		health = "down"
	}
	if _, err := s.reports.StoreMonitorReport(coreWorkerMonitorID, MonitorReportPayload{
		Timestamp: now.Format(time.RFC3339),
		Health:    health,
		Metrics: map[string]any{
			"runner":                "core",
			"type":                  "core-worker-liveness",
			"workers_online":        diag.OnlineCount,
			"workers_stale":         diag.StaleCount,
			"stale_after_seconds":   int(staleAfter.Seconds()),
			"oldest_heartbeat_secs": oldestHeartbeatAge(diag),
		},
	}); err != nil {
		return err
	}

	if !healthy {
		if _, err := s.alerts.ProcessDueDeliveries(selfMonitorDispatchCap); err != nil {
			return err
		}
	}
	return nil
}

func oldestHeartbeatAge(diag CoreWorkerDiagnostics) int64 {
	var oldest int64
	for _, w := range diag.Workers {
		oldest = max(oldest, w.HeartbeatAgeSeconds)
	}
	return oldest
}

func (s *SelfMonitorService) ensureWorkerMonitor() error {
	var existing db.Monitor
	err := s.db.Where("id = ?", coreWorkerMonitorID).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	owner, err := NewIncidentService(s.db, s.logger, s.cfg).ensureCoreOwnerAgent("")
	if err != nil {
		return err
	}
	description := "Orion Core's own monitor worker. Down means checks, heartbeat monitors and alert delivery are not running."
	return s.db.Create(&db.Monitor{
		ID:                       coreWorkerMonitorID,
		AgentID:                  owner.ID,
		Name:                     coreWorkerMonitorName,
		Description:              &description,
		Type:                     "core-worker",
		Lifecycle:                "active",
		Health:                   "unknown",
		ComputedHealth:           "unknown",
		IncidentState:            "unknown",
		ReportingIntervalSeconds: int(selfMonitorInterval.Seconds()),
	}).Error
}

func (s *SelfMonitorService) pingDeadMan(ctx context.Context) {
	if s.cfg.DeadManURL == "" {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.DeadManURL, nil)
	if err != nil {
		s.logger.Error("Invalid ORION_DEADMAN_URL", "error", err)
		return
	}
	req.Header.Set("User-Agent", "orion-core-deadman/1")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Warn("Dead-man ping failed", "error", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		s.logger.Warn("Dead-man ping answered with an error status", "status", resp.StatusCode)
	}
}
