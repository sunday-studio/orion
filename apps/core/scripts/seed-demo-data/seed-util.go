package main

import (
	"encoding/json"
	"path/filepath"
	"time"

	"orion/core/internal/db"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func seedLifecycleSettings(database *gorm.DB, cfg seedConfig, now time.Time) error {
	settings := db.DataLifecycleSettings{
		ID:                1,
		RawReportHotDays:  30,
		ArchiveRawReports: true,
		ArchiveDir:        filepath.Join(filepath.Dir(cfg.dbPath), "archive"),
		RollupsEnabled:    true,
		ArchiveSchedule:   "manual",
		LastRollupRunAt:   ptrTime(now),
		LastArchiveRunAt:  ptrTime(now.Add(-24 * time.Hour)),
		LastArchiveStatus: "success",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	return database.Clauses(clause.OnConflict{UpdateAll: true}).Create(&settings).Error
}

func bulkCreate[T any](database *gorm.DB, rows []T, batchSize int) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	if err := database.CreateInBatches(rows, batchSize).Error; err != nil {
		return 0, err
	}
	return len(rows), nil
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func choose[T any](condition bool, yes T, no T) T {
	if condition {
		return yes
	}
	return no
}

func clamp(value float64, min float64, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
