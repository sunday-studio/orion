package tracestore

import (
	"context"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"time"

	"gorm.io/gorm"
)

const pruneBatch = 1000

// AppRetention is one app's retention policy.
type AppRetention struct {
	AppID         string
	RetentionDays int
	MaxStorageMB  int
}

// PruneResult counts spans removed.
type PruneResult struct {
	Expired   int64
	OverLimit int64
}

// EffectiveDays is how long an app's traces are kept: the app's retention_days, but never longer than
// the instance-wide trace limit (spans are much larger than log lines, so the limit is shorter by
// default). A zero on either side means "not set" and the other one applies.
func EffectiveDays(appDays, limitDays int) int {
	switch {
	case appDays <= 0:
		return limitDays
	case limitDays <= 0:
		return appDays
	}
	return min(appDays, limitDays)
}

// Prune removes spans older than each app's retention and, where a storage cap is set, the oldest
// whole traces beyond it, together with their summaries. Every delete is a short batch so ingest is
// never blocked for long, and freed pages are returned to the disk at the end.
func (s *Store) Prune(policies []AppRetention, now time.Time) (PruneResult, error) {
	var result PruneResult
	for _, p := range policies {
		if p.RetentionDays > 0 {
			cutoff := now.AddDate(0, 0, -p.RetentionDays).UnixNano()
			n, err := s.deleteSpans(true, "app_id = ? AND start_ns < ?", p.AppID, cutoff)
			result.Expired += n
			if err != nil {
				return result, err
			}
		}
		if p.MaxStorageMB > 0 {
			limit := int64(p.MaxStorageMB) << 20
			_, bytes, err := s.CountForApp(p.AppID)
			if err != nil {
				return result, err
			}
			for bytes > limit {
				trimmed, err := s.trimOldestTraces(p.AppID, bytes-limit)
				if err != nil {
					return result, err
				}
				if trimmed == 0 {
					break
				}
				result.OverLimit += trimmed
				if _, bytes, err = s.CountForApp(p.AppID); err != nil {
					return result, err
				}
			}
		}
	}
	s.db.Exec("PRAGMA incremental_vacuum(2000)")
	s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return result, nil
}

// DeleteApp removes every span and summary of an app.
func (s *Store) DeleteApp(appID string) (int64, error) {
	n, err := s.deleteSpans(false, "app_id = ?", appID)
	if err != nil {
		return n, err
	}
	return n, s.db.Exec("DELETE FROM traces WHERE app_id = ?", appID).Error
}

// deleteSpans removes the spans matching where in batches. With refresh the summary of every trace a
// batch touched is rebuilt (or removed when no span is left), so a trace cut by retention stays right.
func (s *Store) deleteSpans(refresh bool, where string, args ...any) (int64, error) {
	var total int64
	for {
		var rows []struct {
			Rowid   int64
			AppID   string
			TraceID string
		}
		var deleted int64
		err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Raw(fmt.Sprintf("SELECT rowid, app_id, trace_id FROM spans WHERE %s LIMIT %d", where, pruneBatch), args...).Scan(&rows).Error; err != nil {
				return err
			}
			if len(rows) == 0 {
				return nil
			}
			rowids := make([]int64, len(rows))
			seen := map[traceKey]bool{}
			var touched []traceKey
			for i, r := range rows {
				rowids[i] = r.Rowid
				if key := (traceKey{r.AppID, r.TraceID}); !seen[key] {
					seen[key] = true
					touched = append(touched, key)
				}
			}
			res := tx.Exec("DELETE FROM spans WHERE rowid IN ?", rowids)
			if res.Error != nil {
				return res.Error
			}
			deleted = res.RowsAffected
			if refresh {
				for _, key := range touched {
					if err := refreshTrace(tx, key.appID, key.traceID); err != nil {
						return err
					}
				}
			}
			return nil
		})
		total += deleted
		if err != nil {
			return total, err
		}
		if len(rows) < pruneBatch {
			return total, nil
		}
	}
}

// trimOldestTraces deletes the app's oldest traces, whole, until at least excess bytes are freed
// (looking at one batch of traces at a time) and returns how many spans it removed. Whole traces go
// together so the cap never leaves a half-trace that looks complete.
func (s *Store) trimOldestTraces(appID string, excess int64) (int64, error) {
	var oldest []string
	if err := s.db.Raw("SELECT trace_id FROM traces WHERE app_id = ? ORDER BY start_ns ASC, trace_id ASC LIMIT 100", appID).Scan(&oldest).Error; err != nil {
		return 0, err
	}
	if len(oldest) == 0 {
		return 0, nil
	}
	var sizes []struct {
		TraceID string
		Bytes   int64
	}
	if err := s.db.Raw("SELECT trace_id, SUM(size) AS bytes FROM spans WHERE app_id = ? AND trace_id IN ? GROUP BY trace_id", appID, oldest).Scan(&sizes).Error; err != nil {
		return 0, err
	}
	bytesOf := make(map[string]int64, len(sizes))
	for _, r := range sizes {
		bytesOf[r.TraceID] = r.Bytes
	}
	var freed int64
	var victims []string
	for _, id := range oldest {
		victims = append(victims, id)
		if freed += bytesOf[id]; freed >= excess {
			break
		}
	}
	var removed int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Exec("DELETE FROM spans WHERE app_id = ? AND trace_id IN ?", appID, victims)
		if res.Error != nil {
			return res.Error
		}
		removed = res.RowsAffected
		return tx.Exec("DELETE FROM traces WHERE app_id = ? AND trace_id IN ?", appID, victims).Error
	})
	return removed, err
}

// RunRetention prunes every app's traces by its retention policy until ctx ends. maxDays returns the
// instance-wide trace retention limit in days (see EffectiveDays); it is read on every run so a changed
// setting applies without a restart.
func RunRetention(ctx context.Context, store *Store, main *gorm.DB, logger *utils.Logger, every time.Duration, maxDays func() int) {
	if every <= 0 {
		every = time.Hour
	}
	run := func(now time.Time) {
		var apps []db.App
		if err := main.Select("id", "retention_days", "max_storage_mb").Find(&apps).Error; err != nil {
			logger.Error("Trace retention could not load apps", "error", err)
			return
		}
		limit := 0
		if maxDays != nil {
			limit = maxDays()
		}
		policies := make([]AppRetention, 0, len(apps))
		for _, app := range apps {
			policies = append(policies, AppRetention{AppID: app.ID, RetentionDays: EffectiveDays(app.RetentionDays, limit), MaxStorageMB: app.MaxStorageMB})
		}
		result, err := store.Prune(policies, now)
		if err != nil {
			logger.Error("Trace retention failed", "error", err)
			return
		}
		if result.Expired+result.OverLimit > 0 {
			logger.Info("Trace retention pruned spans", "expired", result.Expired, "over_storage_cap", result.OverLimit)
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
