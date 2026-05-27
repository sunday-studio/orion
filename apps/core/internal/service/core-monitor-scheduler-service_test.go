package service

import (
	"errors"
	"testing"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/logging"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestClaimDueCoreMonitorConfigsClaimsDueConfigOnce(t *testing.T) {
	database := openCoreMonitorSchedulerTestDatabase(t)
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	insertCoreMonitorSchedulerAgent(t, database, "agent-core")
	insertCoreMonitorSchedulerMonitor(t, database, "monitor-due", "agent-core", "active")
	insertCoreMonitorSchedulerConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-due",
		Kind:            "http",
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		NextRunAt:       now.Add(-time.Minute),
	})

	service := NewCoreMonitorSchedulerService(database, logging.NewLogger())
	claimed, err := service.ClaimDueCoreMonitorConfigs(ClaimDueCoreMonitorConfigsRequest{
		LeaseOwner:    "worker-a",
		Limit:         10,
		LeaseDuration: 2 * time.Minute,
		Now:           now,
	})
	if err != nil {
		t.Fatalf("ClaimDueCoreMonitorConfigs() error = %v", err)
	}
	if len(claimed) != 1 || claimed[0].MonitorID != "monitor-due" {
		t.Fatalf("claimed = %+v, want monitor-due", claimed)
	}
	if claimed[0].LeaseOwner != "worker-a" || claimed[0].LeaseExpiresAt == nil || !claimed[0].LeaseExpiresAt.Equal(now.Add(2*time.Minute)) {
		t.Fatalf("claimed lease = owner:%q expires:%v, want worker-a at %v", claimed[0].LeaseOwner, claimed[0].LeaseExpiresAt, now.Add(2*time.Minute))
	}

	duplicate, err := service.ClaimDueCoreMonitorConfigs(ClaimDueCoreMonitorConfigsRequest{
		LeaseOwner:    "worker-b",
		Limit:         10,
		LeaseDuration: 2 * time.Minute,
		Now:           now,
	})
	if err != nil {
		t.Fatalf("second ClaimDueCoreMonitorConfigs() error = %v", err)
	}
	if len(duplicate) != 0 {
		t.Fatalf("second claimed = %+v, want no duplicate claim", duplicate)
	}
}

func TestClaimDueCoreMonitorConfigsRecoversOverdueUnleasedConfig(t *testing.T) {
	database := openCoreMonitorSchedulerTestDatabase(t)
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	insertCoreMonitorSchedulerAgent(t, database, "agent-core")
	insertCoreMonitorSchedulerMonitor(t, database, "monitor-overdue", "agent-core", "active")
	insertCoreMonitorSchedulerConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-overdue",
		Kind:            "http",
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		NextRunAt:       now.Add(-6 * time.Hour),
	})

	service := NewCoreMonitorSchedulerService(database, logging.NewLogger())
	claimed, err := service.ClaimDueCoreMonitorConfigs(ClaimDueCoreMonitorConfigsRequest{
		LeaseOwner:    "worker-after-restart",
		LeaseDuration: time.Minute,
		Now:           now,
	})
	if err != nil {
		t.Fatalf("ClaimDueCoreMonitorConfigs() error = %v", err)
	}
	if len(claimed) != 1 || claimed[0].MonitorID != "monitor-overdue" {
		t.Fatalf("claimed = %+v, want overdue monitor after restart", claimed)
	}
}

func TestClaimDueCoreMonitorConfigsReclaimsExpiredLease(t *testing.T) {
	database := openCoreMonitorSchedulerTestDatabase(t)
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-time.Second)
	insertCoreMonitorSchedulerAgent(t, database, "agent-core")
	insertCoreMonitorSchedulerMonitor(t, database, "monitor-expired", "agent-core", "active")
	insertCoreMonitorSchedulerConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-expired",
		Kind:            "http",
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		NextRunAt:       now.Add(-time.Minute),
		LeaseOwner:      "dead-worker",
		LeaseExpiresAt:  &expiredAt,
	})

	service := NewCoreMonitorSchedulerService(database, logging.NewLogger())
	claimed, err := service.ClaimDueCoreMonitorConfigs(ClaimDueCoreMonitorConfigsRequest{
		LeaseOwner:    "worker-b",
		LeaseDuration: 90 * time.Second,
		Now:           now,
	})
	if err != nil {
		t.Fatalf("ClaimDueCoreMonitorConfigs() error = %v", err)
	}
	if len(claimed) != 1 || claimed[0].LeaseOwner != "worker-b" {
		t.Fatalf("claimed = %+v, want worker-b to reclaim expired lease", claimed)
	}
}

func TestClaimDueCoreMonitorConfigsSkipsPausedAndInactiveMonitors(t *testing.T) {
	database := openCoreMonitorSchedulerTestDatabase(t)
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	insertCoreMonitorSchedulerAgent(t, database, "agent-core")

	insertCoreMonitorSchedulerMonitor(t, database, "monitor-paused", "agent-core", "active")
	insertCoreMonitorSchedulerConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-paused",
		Kind:            "http",
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		Paused:          true,
		NextRunAt:       now.Add(-time.Minute),
	})

	insertCoreMonitorSchedulerMonitor(t, database, "monitor-disabled", "agent-core", "disabled")
	insertCoreMonitorSchedulerConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-disabled",
		Kind:            "http",
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		NextRunAt:       now.Add(-time.Minute),
	})

	service := NewCoreMonitorSchedulerService(database, logging.NewLogger())
	claimed, err := service.ClaimDueCoreMonitorConfigs(ClaimDueCoreMonitorConfigsRequest{
		LeaseOwner: "worker-a",
		Now:        now,
	})
	if err != nil {
		t.Fatalf("ClaimDueCoreMonitorConfigs() error = %v", err)
	}
	if len(claimed) != 0 {
		t.Fatalf("claimed = %+v, want paused and disabled monitors skipped", claimed)
	}
}

func TestClaimDueCoreMonitorConfigsClaimsResumedMonitor(t *testing.T) {
	database := openCoreMonitorSchedulerTestDatabase(t)
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	insertCoreMonitorSchedulerAgent(t, database, "agent-core")
	insertCoreMonitorSchedulerMonitor(t, database, "monitor-resumed", "agent-core", "active")
	insertCoreMonitorSchedulerConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-resumed",
		Kind:            "http",
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		Paused:          true,
		NextRunAt:       now.Add(-time.Minute),
	})

	service := NewCoreMonitorSchedulerService(database, logging.NewLogger())
	pausedClaim, err := service.ClaimDueCoreMonitorConfigs(ClaimDueCoreMonitorConfigsRequest{
		LeaseOwner: "worker-paused",
		Now:        now,
	})
	if err != nil {
		t.Fatalf("paused ClaimDueCoreMonitorConfigs() error = %v", err)
	}
	if len(pausedClaim) != 0 {
		t.Fatalf("paused claimed = %+v, want no claim", pausedClaim)
	}

	if err := database.Model(&db.CoreMonitorConfig{}).
		Where("monitor_id = ?", "monitor-resumed").
		Update("paused", false).Error; err != nil {
		t.Fatalf("resume monitor: %v", err)
	}

	resumedClaim, err := service.ClaimDueCoreMonitorConfigs(ClaimDueCoreMonitorConfigsRequest{
		LeaseOwner:    "worker-resumed",
		LeaseDuration: time.Minute,
		Now:           now,
	})
	if err != nil {
		t.Fatalf("resumed ClaimDueCoreMonitorConfigs() error = %v", err)
	}
	if len(resumedClaim) != 1 || resumedClaim[0].MonitorID != "monitor-resumed" || resumedClaim[0].LeaseOwner != "worker-resumed" {
		t.Fatalf("resumed claimed = %+v, want monitor-resumed leased by worker-resumed", resumedClaim)
	}
}

func TestCompleteCoreMonitorCheckSchedulesNextRunAndClearsLease(t *testing.T) {
	database := openCoreMonitorSchedulerTestDatabase(t)
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	finishedAt := now.Add(15 * time.Second)
	insertCoreMonitorSchedulerAgent(t, database, "agent-core")
	insertCoreMonitorSchedulerMonitor(t, database, "monitor-complete", "agent-core", "active")
	insertCoreMonitorSchedulerConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-complete",
		Kind:            "http",
		IntervalSeconds: 120,
		TimeoutSeconds:  10,
		NextRunAt:       now.Add(-time.Minute),
	})

	service := NewCoreMonitorSchedulerService(database, logging.NewLogger())
	if _, err := service.ClaimDueCoreMonitorConfigs(ClaimDueCoreMonitorConfigsRequest{
		LeaseOwner:    "worker-a",
		LeaseDuration: time.Minute,
		Now:           now,
	}); err != nil {
		t.Fatalf("ClaimDueCoreMonitorConfigs() error = %v", err)
	}

	completed, err := service.CompleteCoreMonitorCheck(CompleteCoreMonitorCheckRequest{
		MonitorID:  "monitor-complete",
		LeaseOwner: "worker-a",
		FinishedAt: finishedAt,
		Success:    true,
	})
	if err != nil {
		t.Fatalf("CompleteCoreMonitorCheck() error = %v", err)
	}
	if completed.LeaseOwner != "" || completed.LeaseExpiresAt != nil {
		t.Fatalf("completed lease = owner:%q expires:%v, want cleared", completed.LeaseOwner, completed.LeaseExpiresAt)
	}
	if completed.LastRunAt == nil || !completed.LastRunAt.Equal(finishedAt) {
		t.Fatalf("last_run_at = %v, want %v", completed.LastRunAt, finishedAt)
	}
	if completed.LastSuccessAt == nil || !completed.LastSuccessAt.Equal(finishedAt) {
		t.Fatalf("last_success_at = %v, want %v", completed.LastSuccessAt, finishedAt)
	}
	if !completed.NextRunAt.Equal(finishedAt.Add(120 * time.Second)) {
		t.Fatalf("next_run_at = %v, want %v", completed.NextRunAt, finishedAt.Add(120*time.Second))
	}
}

func TestCompleteCoreMonitorCheckRejectsExpiredLease(t *testing.T) {
	database := openCoreMonitorSchedulerTestDatabase(t)
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-time.Second)
	insertCoreMonitorSchedulerAgent(t, database, "agent-core")
	insertCoreMonitorSchedulerMonitor(t, database, "monitor-expired-complete", "agent-core", "active")
	insertCoreMonitorSchedulerConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-expired-complete",
		Kind:            "http",
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		NextRunAt:       now.Add(-time.Minute),
		LeaseOwner:      "worker-a",
		LeaseExpiresAt:  &expiredAt,
	})

	service := NewCoreMonitorSchedulerService(database, logging.NewLogger())
	if _, err := service.CompleteCoreMonitorCheck(CompleteCoreMonitorCheckRequest{
		MonitorID:  "monitor-expired-complete",
		LeaseOwner: "worker-a",
		FinishedAt: now,
		Success:    false,
	}); !errors.Is(err, ErrCoreMonitorLeaseNotHeld) {
		t.Fatalf("CompleteCoreMonitorCheck() error = %v, want ErrCoreMonitorLeaseNotHeld", err)
	}
}

func openCoreMonitorSchedulerTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	return database
}

func insertCoreMonitorSchedulerAgent(t *testing.T, database *gorm.DB, agentID string) {
	t.Helper()

	agent := db.Agent{
		ID:        agentID,
		MachineId: agentID + "-machine",
		Name:      agentID,
		OS:        "linux",
		Arch:      "arm64",
		Token:     agentID + "-token",
		LastSeen:  time.Now(),
	}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
}

func insertCoreMonitorSchedulerMonitor(t *testing.T, database *gorm.DB, monitorID string, agentID string, lifecycle string) {
	t.Helper()

	monitor := db.Monitor{
		ID:                       monitorID,
		AgentID:                  agentID,
		Name:                     monitorID,
		Type:                     "http",
		Lifecycle:                lifecycle,
		Health:                   "unknown",
		ComputedHealth:           "unknown",
		ReportingIntervalSeconds: 60,
	}
	if err := database.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}
}

func insertCoreMonitorSchedulerConfig(t *testing.T, database *gorm.DB, config db.CoreMonitorConfig) {
	t.Helper()

	if config.ConfigJSON == "" {
		config.ConfigJSON = "{}"
	}
	if config.SecretRefJSON == "" {
		config.SecretRefJSON = "{}"
	}
	if config.IntervalSeconds == 0 {
		config.IntervalSeconds = 60
	}
	if config.TimeoutSeconds == 0 {
		config.TimeoutSeconds = 10
	}
	if config.CreatedAt.IsZero() {
		config.CreatedAt = time.Now().UTC()
	}
	if config.UpdatedAt.IsZero() {
		config.UpdatedAt = config.CreatedAt
	}
	if err := database.Create(&config).Error; err != nil {
		t.Fatalf("create core monitor config: %v", err)
	}
}
