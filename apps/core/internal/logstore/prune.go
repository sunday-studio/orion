package logstore

import (
	"fmt"
	"time"
)

const pruneBatch = 1000

// AppRetention is one app's retention policy.
type AppRetention struct {
	AppID         string
	RetentionDays int
	MaxStorageMB  int
}

// PruneResult counts rows removed.
type PruneResult struct {
	Expired   int64
	OverLimit int64
}

// Prune removes entries older than each app's retention and, where a storage cap is
// set, the oldest entries beyond it. Every delete is a short batch so ingest is
// never blocked for long. Unknown apps' entries are left to DeleteApp.
func (s *Store) Prune(policies []AppRetention, now time.Time) (PruneResult, error) {
	var result PruneResult
	for _, p := range policies {
		if p.RetentionDays > 0 {
			cutoff := now.AddDate(0, 0, -p.RetentionDays).UnixNano()
			n, err := s.deleteBatched("app_id = ? AND ts < ?", p.AppID, cutoff)
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
				trimmed, err := s.trimOldest(p.AppID, bytes-limit)
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

// DeleteApp removes every entry of an app.
func (s *Store) DeleteApp(appID string) (int64, error) {
	return s.deleteBatched("app_id = ?", appID)
}

func (s *Store) deleteBatched(where string, args ...any) (int64, error) {
	var total int64
	statement := fmt.Sprintf("DELETE FROM log_entries WHERE rowid IN (SELECT rowid FROM log_entries WHERE %s LIMIT %d)", where, pruneBatch)
	for {
		res := s.db.Exec(statement, args...)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < pruneBatch {
			return total, nil
		}
	}
}

// trimOldest deletes the app's oldest entries until at least excess bytes are freed
// (looking at up to one batch at a time) and returns how many rows it removed.
func (s *Store) trimOldest(appID string, excess int64) (int64, error) {
	var oldest []struct {
		Rowid int64
		Size  int64
	}
	if err := s.db.Raw("SELECT rowid, size FROM log_entries WHERE app_id = ? ORDER BY ts ASC LIMIT ?", appID, pruneBatch).Scan(&oldest).Error; err != nil {
		return 0, err
	}
	var freed int64
	var rowids []int64
	for _, row := range oldest {
		rowids = append(rowids, row.Rowid)
		if freed += row.Size; freed >= excess {
			break
		}
	}
	if len(rowids) == 0 {
		return 0, nil
	}
	res := s.db.Exec("DELETE FROM log_entries WHERE rowid IN ?", rowids)
	return res.RowsAffected, res.Error
}
