package logstore

import (
	"context"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"time"

	"gorm.io/gorm"
)

// RunRetention prunes every app's logs by its retention policy until ctx ends.
func RunRetention(ctx context.Context, store *Store, main *gorm.DB, logger *utils.Logger, every time.Duration) {
	if every <= 0 {
		every = time.Hour
	}
	run := func(now time.Time) {
		var apps []db.App
		if err := main.Select("id", "retention_days", "max_storage_mb").Find(&apps).Error; err != nil {
			logger.Error("Log retention could not load apps", "error", err)
			return
		}
		policies := make([]AppRetention, 0, len(apps))
		for _, app := range apps {
			policies = append(policies, AppRetention{AppID: app.ID, RetentionDays: app.RetentionDays, MaxStorageMB: app.MaxStorageMB})
		}
		result, err := store.Prune(policies, now)
		if err != nil {
			logger.Error("Log retention failed", "error", err)
			return
		}
		if result.Expired+result.OverLimit > 0 {
			logger.Info("Log retention pruned entries", "expired", result.Expired, "over_storage_cap", result.OverLimit)
		}
	}
	run(time.Now().UTC())
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			run(now.UTC())
		}
	}
}
