package db

import (
	"context"
	"orion/core/internal/logging"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	retentionBatchSize = 1000
	retentionInterval  = time.Hour
)

// StartRetention prunes reports older than days from the main DB on startup and then hourly.
// Pruned rows are copied to archive first (when non-nil). Monitor reports are also folded
// into monitor_daily_rollups so long-range uptime survives pruning. days <= 0 disables it.
func StartRetention(ctx context.Context, main, archive *gorm.DB, days int, log *logging.Logger) {
	if days <= 0 {
		log.Info("Report retention disabled")
		return
	}

	run := func() {
		cutoff := time.Now().AddDate(0, 0, -days)
		if err := pruneMonitorReports(main, archive, cutoff, log); err != nil {
			log.Error("Monitor report retention failed", "error", err)
		}
		if err := pruneAgentReports(main, archive, cutoff, log); err != nil {
			log.Error("Agent report retention failed", "error", err)
		}
		// Return WAL pages to the OS after large deletes.
		main.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	}

	go func() {
		run()
		ticker := time.NewTicker(retentionInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

type rollupKey struct{ monitorID, date string }

func pruneMonitorReports(main, archive *gorm.DB, cutoff time.Time, log *logging.Logger) error {
	total := 0
	for {
		var batch []MonitorReport
		if err := main.Where("created_at < ?", cutoff).Order("created_at ASC").Limit(retentionBatchSize).Find(&batch).Error; err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}

		// Archive first and idempotently: if the delete below fails, a retry re-inserts nothing new.
		if archive != nil {
			if err := archive.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(batch, 200).Error; err != nil {
				return err
			}
		}

		type counts struct{ up, total int }
		agg := make(map[rollupKey]*counts)
		ids := make([]string, 0, len(batch))
		for _, r := range batch {
			ids = append(ids, r.ID)
			k := rollupKey{r.MonitorID, r.CreatedAt.Format("2006-01-02")}
			if agg[k] == nil {
				agg[k] = &counts{}
			}
			agg[k].total++
			if r.Health == "up" {
				agg[k].up++
			}
		}

		// Rollup increment and delete commit together, so rows are never counted twice.
		err := main.Transaction(func(tx *gorm.DB) error {
			for k, c := range agg {
				row := MonitorDailyRollup{MonitorID: k.monitorID, Date: k.date, Up: c.up, Total: c.total}
				if err := tx.Clauses(clause.OnConflict{
					Columns: []clause.Column{{Name: "monitor_id"}, {Name: "date"}},
					DoUpdates: clause.Assignments(map[string]any{
						"up":    gorm.Expr("monitor_daily_rollups.up + excluded.up"),
						"total": gorm.Expr("monitor_daily_rollups.total + excluded.total"),
					}),
				}).Create(&row).Error; err != nil {
					return err
				}
			}
			return tx.Where("id IN ?", ids).Delete(&MonitorReport{}).Error
		})
		if err != nil {
			return err
		}
		total += len(batch)
	}
	if total > 0 {
		log.Info("Pruned monitor reports", "count", total, "archived", archive != nil)
	}
	return nil
}

func pruneAgentReports(main, archive *gorm.DB, cutoff time.Time, log *logging.Logger) error {
	total := 0
	for {
		var batch []AgentReport
		if err := main.Where("created_at < ?", cutoff).Order("created_at ASC").Limit(retentionBatchSize).Find(&batch).Error; err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}

		if archive != nil {
			if err := archive.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(batch, 200).Error; err != nil {
				return err
			}
		}

		ids := make([]string, 0, len(batch))
		for _, r := range batch {
			ids = append(ids, r.ID)
		}
		if err := main.Where("id IN ?", ids).Delete(&AgentReport{}).Error; err != nil {
			return err
		}
		total += len(batch)
	}
	if total > 0 {
		log.Info("Pruned agent reports", "count", total, "archived", archive != nil)
	}
	return nil
}
