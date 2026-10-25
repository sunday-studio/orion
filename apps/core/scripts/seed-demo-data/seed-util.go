package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
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

// monitorReportStep keeps hourly history but samples every 10 minutes over the
// last 48 hours so the day and week response-time charts have enough points.
func monitorReportStep(interval time.Duration, t, now time.Time) time.Duration {
	dense := 10 * time.Minute
	if now.Sub(t) <= 48*time.Hour && interval > dense {
		return dense
	}
	return interval
}

// responseTimeMs is a deterministic, smoothly varying latency per monitor:
// a per-monitor base, a slow daily wave and small jitter.
func responseTimeMs(key string, t time.Time) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	base := 40 + float64(h.Sum32()%260)
	wave := 1 + 0.3*math.Sin(float64(t.Unix())/(3*3600))
	jh := fnv.New32a()
	_, _ = jh.Write([]byte(fmt.Sprintf("%s-%d", key, t.Unix())))
	jitter := 0.9 + float64(jh.Sum32()%200)/1000
	return int(base * wave * jitter)
}
