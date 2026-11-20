package tracestore

import (
	"context"
	"sort"
	"time"
)

// FacetLimit bounds each list a facets call returns.
const FacetLimit = 50

// Count is one facet value with how many traces carry it.
type Count struct {
	Value string
	Count int64
}

// Facets are the choices a filter bar offers for the traces matching a query.
type Facets struct {
	// Total is how many traces match every filter, the status one included: what the search returns.
	Total int64
	// Services are the services of the matching traces' root spans, most traces first.
	Services []Count
	// Names are the root span names, most traces first.
	Names []Count
	// Status counts traces without (ok) and with (error) a failed span, whatever status filter is set,
	// so a filter bar can show both choices.
	Status []Count
}

// Facets describes the traces matching q (its cursor and limit are ignored). Total, Services and Names
// follow every filter, status included, so they agree with the search. Status counts the traces that match
// all filters except status, so both choices stay visible: Total is the ok or the error count when a status
// is picked, and their sum when not. Services and Names list at most FacetLimit values each.
func (s *Store) Facets(q Query) (Facets, error) {
	q.Cursor, q.Limit = "", 0
	status := q.Status
	q.Status = ""
	where, args, err := q.where()
	if err != nil {
		return Facets{}, err
	}
	db := s.dbq(q.Ctx)
	var out Facets
	var totals struct{ N, Errors int64 }
	if err := db.Raw("SELECT COUNT(*) AS n, COALESCE(SUM(t.error_count > 0), 0) AS errors FROM traces t"+where, args...).Scan(&totals).Error; err != nil {
		return Facets{}, err
	}
	out.Status = []Count{{Value: "ok", Count: totals.N - totals.Errors}, {Value: "error", Count: totals.Errors}}
	out.Total = totals.N
	if status != "" { // Total and the lists follow the status the caller picked
		q.Status = status
		if where, args, err = q.where(); err != nil {
			return Facets{}, err
		}
		out.Total = out.Status[map[string]int{"ok": 0, "error": 1}[status]].Count
	}
	for _, list := range []struct {
		column string
		into   *[]Count
	}{{"service_name", &out.Services}, {"root_name", &out.Names}} {
		var rows []struct {
			Value string
			N     int64
		}
		clause := where
		if clause == "" {
			clause = " WHERE t." + list.column + " <> ''"
		} else {
			clause += " AND t." + list.column + " <> ''"
		}
		if err := db.Raw("SELECT t."+list.column+" AS value, COUNT(*) AS n FROM traces t"+clause+" GROUP BY t."+list.column+" ORDER BY n DESC, value ASC LIMIT ?", append(append([]any{}, args...), FacetLimit)...).Scan(&rows).Error; err != nil {
			return Facets{}, err
		}
		for _, r := range rows {
			*list.into = append(*list.into, Count{Value: r.Value, Count: r.N})
		}
	}
	return out, nil
}

// SummaryRowLimit bounds how many traces a latency percentile is computed from.
const SummaryRowLimit = 50_000

// ServiceSummary describes the traces of a set of apps over a window.
type ServiceSummary struct {
	TraceCount int64
	// ErrorCount is the traces with at least one failed span.
	ErrorCount int64
	// RootedCount is the traces whose root span arrived: only those have a root-span latency.
	RootedCount int64
	// P50, P95 and P99 are root-span durations in nanoseconds (nearest rank), zero when RootedCount is zero.
	P50, P95, P99 int64
	// Sampled is true when more than SummaryRowLimit traces matched and the percentiles use the newest ones.
	Sampled bool
}

// Summarize counts the traces of the apps that started in [from, to] and derives root-span latency
// percentiles. Counts are exact; percentiles read at most SummaryRowLimit durations, so cost is bounded.
func (s *Store) Summarize(ctx context.Context, appIDs []string, from, to time.Time) (ServiceSummary, error) {
	var out ServiceSummary
	if len(appIDs) == 0 {
		return out, nil
	}
	db := s.dbq(ctx)
	var totals struct{ N, Errors, Rooted int64 }
	if err := db.Raw(`SELECT COUNT(*) AS n, COALESCE(SUM(error_count > 0), 0) AS errors, COALESCE(SUM(has_root), 0) AS rooted
		FROM traces WHERE app_id IN ? AND start_ns >= ? AND start_ns <= ?`, appIDs, from.UnixNano(), to.UnixNano()).Scan(&totals).Error; err != nil {
		return out, err
	}
	out.TraceCount, out.ErrorCount, out.RootedCount = totals.N, totals.Errors, totals.Rooted
	if totals.Rooted == 0 {
		return out, nil
	}
	var durations []int64
	if err := db.Raw(`SELECT duration_ns FROM traces WHERE app_id IN ? AND has_root = 1 AND start_ns >= ? AND start_ns <= ?
		ORDER BY start_ns DESC LIMIT ?`, appIDs, from.UnixNano(), to.UnixNano(), SummaryRowLimit).Scan(&durations).Error; err != nil {
		return out, err
	}
	out.Sampled = totals.Rooted > SummaryRowLimit
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	out.P50, out.P95, out.P99 = percentile(durations, 50), percentile(durations, 95), percentile(durations, 99)
	return out, nil
}

// percentile is the nearest-rank percentile of sorted values.
func percentile(sorted []int64, pct int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := (pct*len(sorted) + 99) / 100 // ceil(pct/100 * n) in integers
	return sorted[min(max(rank, 1), len(sorted))-1]
}
