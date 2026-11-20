package tracestore

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// insert writes the spans in one transaction, then rebuilds the summary of every trace the batch
// touched, and returns how many span rows were really stored (a repeated span id is ignored).
//
// The summary is recomputed from the stored spans instead of being adjusted by deltas. That keeps
// it right whatever the arrival order (a late root span, a child before its parent), under exporter
// retries that resend spans, and after retention removes part of a trace.
func (s *Store) insert(spans []Span) (int64, error) {
	var stored int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		stored = 0
		for start := 0; start < len(spans); {
			// A statement holds at most insertChunk rows and about insertChunkBytes of arguments, so a
			// few very large spans never become one huge statement.
			sql := "INSERT OR IGNORE INTO spans (app_id, trace_id, span_id, parent_span_id, name, kind, start_ns, duration_ns, status_code, status_message, service_name, attributes, resource, events, links, ingested_at, size) VALUES "
			args := make([]any, 0, insertChunk*17)
			rows, bytes := 0, 0
			for start < len(spans) {
				row, size := encodeRow(spans[start])
				if !fitsChunk(rows, bytes, size) {
					break
				}
				if rows > 0 {
					sql += ","
				}
				sql += "(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"
				args = append(args, row...)
				rows++
				bytes += size
				start++
			}
			res := tx.Exec(sql, args...)
			if res.Error != nil {
				return res.Error
			}
			stored += res.RowsAffected
		}
		if stored == 0 {
			return nil
		}
		seen := map[traceKey]bool{}
		for _, sp := range spans {
			key := traceKey{sp.AppID, sp.TraceID}
			if seen[key] {
				continue
			}
			seen[key] = true
			if err := refreshTrace(tx, sp.AppID, sp.TraceID); err != nil {
				return err
			}
		}
		return nil
	})
	return stored, err
}

// fitsChunk says whether one more row of next bytes belongs in a statement already holding rows rows
// and bytes bytes. A statement always takes its first row, however large.
func fitsChunk(rows, bytes, next int) bool {
	return rows == 0 || (rows < insertChunk && bytes+next <= insertChunkBytes)
}

// encodeRow turns a span into the insert arguments and the bytes it stores. The size is measured from
// the encoded columns, after redaction, so the stored size column and the storage cap are real.
func encodeRow(sp Span) ([]any, int) {
	attributes, resource, events, links := encodeJSON(sp.Attributes), encodeJSON(sp.Resource), encodeList(sp.Events), encodeList(sp.Links)
	size := storedSize(sp, len(attributes), len(resource), len(events), len(links))
	return []any{sp.AppID, sp.TraceID, sp.SpanID, sp.ParentSpanID, sp.Name, sp.Kind, sp.StartNano, sp.DurationNano, sp.StatusCode, sp.StatusMessage,
		sp.ServiceName, attributes, resource, events, links, sp.IngestedAt.UnixNano(), size}, size
}

type traceKey struct{ appID, traceID string }

// refreshTrace rebuilds the summary row of one (app, trace) from its spans, or removes the row when
// no span is left.
//
//   - Root: the earliest span with no parent. Its name, service, start and duration describe the trace.
//   - No root (orphans only, the root is still in flight or came from another app): the earliest span names
//     the trace and the duration is the extent of all spans.
func refreshTrace(tx *gorm.DB, appID, traceID string) error {
	var agg struct {
		N        int64
		Errors   int64
		MinStart int64
		MaxEnd   int64
	}
	if err := tx.Raw(`SELECT COUNT(*) AS n, COALESCE(SUM(status_code = 2), 0) AS errors, COALESCE(MIN(start_ns), 0) AS min_start, COALESCE(MAX(start_ns + duration_ns), 0) AS max_end
		FROM spans WHERE trace_id = ? AND app_id = ?`, traceID, appID).Scan(&agg).Error; err != nil {
		return err
	}
	if agg.N == 0 {
		return tx.Exec("DELETE FROM traces WHERE app_id = ? AND trace_id = ?", appID, traceID).Error
	}
	var head struct {
		Name         string
		ServiceName  string
		StartNs      int64
		DurationNs   int64
		ParentSpanID string
	}
	if err := tx.Raw(`SELECT name, service_name, start_ns, duration_ns, parent_span_id FROM spans WHERE trace_id = ? AND app_id = ?
		ORDER BY (parent_span_id <> '') ASC, start_ns ASC, span_id ASC LIMIT 1`, traceID, appID).Scan(&head).Error; err != nil {
		return err
	}
	hasRoot := head.ParentSpanID == ""
	start, duration := head.StartNs, head.DurationNs
	if !hasRoot {
		start, duration = agg.MinStart, agg.MaxEnd-agg.MinStart
	}
	rooted := 0
	if hasRoot {
		rooted = 1
	}
	return tx.Exec(`INSERT INTO traces (app_id, trace_id, root_name, service_name, start_ns, duration_ns, span_count, error_count, has_root, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(app_id, trace_id) DO UPDATE SET root_name = excluded.root_name, service_name = excluded.service_name, start_ns = excluded.start_ns,
		  duration_ns = excluded.duration_ns, span_count = excluded.span_count, error_count = excluded.error_count, has_root = excluded.has_root, updated_at = excluded.updated_at`,
		appID, traceID, head.Name, head.ServiceName, start, duration, agg.N, agg.Errors, rooted, time.Now().UnixNano()).Error
}

func encodeJSON(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

func encodeList[T any](items []T) string {
	if len(items) == 0 {
		return ""
	}
	b, err := json.Marshal(items)
	if err != nil {
		return ""
	}
	return string(b)
}
