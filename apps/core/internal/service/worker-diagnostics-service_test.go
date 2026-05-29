package service

import (
	"context"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWorkerDiagnosticsRecordsFreshAndStaleWorkers(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	diagnosticsService := NewWorkerDiagnosticsService(database, logging.NewLogger())
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)

	if err := diagnosticsService.RecordHeartbeat(context.Background(), WorkerHeartbeat{
		WorkerID:        "worker-fresh",
		Hostname:        "core-host",
		Status:          "running",
		Version:         "test",
		StartedAt:       now.Add(-time.Minute),
		LastHeartbeatAt: now,
	}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}
	if err := database.Create(&db.CoreWorkerStatus{
		WorkerID:        "worker-stale",
		ProcessKind:     CoreMonitorWorkerProcessKind,
		Hostname:        "core-host",
		Status:          "running",
		Version:         "test",
		StartedAt:       now.Add(-time.Hour),
		LastHeartbeatAt: now.Add(-2 * time.Minute),
		CreatedAt:       now.Add(-time.Hour),
		UpdatedAt:       now.Add(-2 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("create stale row: %v", err)
	}

	diagnostics, err := diagnosticsService.GetDiagnostics(context.Background(), time.Minute, now)
	if err != nil {
		t.Fatalf("get diagnostics: %v", err)
	}

	if diagnostics.Status != "degraded" || diagnostics.WorkerCount != 2 || diagnostics.OnlineCount != 1 || diagnostics.StaleCount != 1 {
		t.Fatalf("diagnostics = %+v, want degraded with one online and one stale", diagnostics)
	}
	if diagnostics.Workers[0].WorkerID != "worker-fresh" || diagnostics.Workers[0].Health != "online" {
		t.Fatalf("first worker = %+v, want fresh worker online", diagnostics.Workers[0])
	}
	if diagnostics.Workers[1].WorkerID != "worker-stale" || diagnostics.Workers[1].Health != "stale" {
		t.Fatalf("second worker = %+v, want stale worker stale", diagnostics.Workers[1])
	}
}
