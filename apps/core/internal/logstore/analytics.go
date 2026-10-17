package logstore

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// HistogramBucket is the count per severity in one time bucket.
type HistogramBucket struct {
	Start    time.Time
	Total    int64
	Severity map[string]int
}

// Histogram counts entries per severity over [from, to] in about `buckets` equal
// buckets (the width is rounded up to a whole second). Empty buckets are included so
// charts have no gaps. Filters are the same as List; the cursor is ignored.
func (s *Store) Histogram(q Query, from, to time.Time, buckets int) (int, []HistogramBucket, error) {
	if buckets <= 0 {
		buckets = 60
	}
	buckets = min(buckets, 500)
	if !to.After(from) {
		return 0, nil, fmt.Errorf("%w: to must be after from", ErrBadFilter)
	}
	width := time.Duration(int64(to.Sub(from)) / int64(buckets)).Truncate(time.Second)
	if width < time.Second {
		width = time.Second
	}
	q.Cursor, q.From, q.To = "", from, to
	where, args, err := q.where()
	if err != nil {
		return 0, nil, err
	}
	var rows []struct {
		Bucket   int64
		Severity string
		N        int
	}
	w := int64(width)
	if err := s.db.Raw(fmt.Sprintf("SELECT (ts / %d) * %d AS bucket, severity, COUNT(*) AS n FROM log_entries%s GROUP BY bucket, severity", w, w, where), args...).Scan(&rows).Error; err != nil {
		return 0, nil, err
	}
	byStart := map[int64]*HistogramBucket{}
	for _, r := range rows {
		b := byStart[r.Bucket]
		if b == nil {
			b = &HistogramBucket{Start: time.Unix(0, r.Bucket).UTC(), Severity: map[string]int{}}
			byStart[r.Bucket] = b
		}
		b.Severity[r.Severity] += r.N
		b.Total += int64(r.N)
	}
	var out []HistogramBucket
	for start := (from.UnixNano() / w) * w; start <= to.UnixNano(); start += w {
		if b := byStart[start]; b != nil {
			out = append(out, *b)
		} else {
			out = append(out, HistogramBucket{Start: time.Unix(0, start).UTC(), Severity: map[string]int{}})
		}
	}
	return int(width / time.Second), out, nil
}

// FieldStat describes one attribute key.
type FieldStat struct {
	Key          string
	Type         string
	Cardinality  int
	SampleValues []string
	LastSeen     time.Time
}

const (
	fieldSampleRows   = 2000
	fieldMaxValues    = 1000
	fieldSampleValues = 5
)

// FieldCatalog summarises the attribute keys seen in an app's most recent entries:
// dominant type, approximate cardinality (capped) and a few sample values. It reads a
// bounded sample on demand, so it is cheap and always current.
func (s *Store) FieldCatalog(appID string) ([]FieldStat, error) {
	var rows []struct {
		Ts         int64
		Attributes string
	}
	if err := s.db.Raw("SELECT ts, attributes FROM log_entries WHERE app_id = ? AND attributes <> '' ORDER BY ts DESC LIMIT ?", appID, fieldSampleRows).Scan(&rows).Error; err != nil {
		return nil, err
	}
	type acc struct {
		types  map[string]int
		values map[string]struct{}
		last   int64
	}
	fields := map[string]*acc{}
	for _, row := range rows {
		attrs := decodeObject(row.Attributes)
		for key, value := range attrs {
			a := fields[key]
			if a == nil {
				a = &acc{types: map[string]int{}, values: map[string]struct{}{}}
				fields[key] = a
			}
			a.types[jsonType(value)]++
			if len(a.values) < fieldMaxValues {
				a.values[fmt.Sprint(value)] = struct{}{}
			}
			a.last = max(a.last, row.Ts)
		}
	}
	out := make([]FieldStat, 0, len(fields))
	for key, a := range fields {
		stat := FieldStat{Key: key, Cardinality: len(a.values), LastSeen: time.Unix(0, a.last).UTC()}
		best, bestN := "", 0
		for t, n := range a.types {
			if n > bestN {
				best, bestN = t, n
			}
		}
		stat.Type = best
		if len(a.types) > 1 {
			stat.Type = "mixed"
		}
		if stat.Type != "object" && stat.Type != "array" {
			for v := range a.values {
				stat.SampleValues = append(stat.SampleValues, truncate(v, 80))
			}
			sort.Strings(stat.SampleValues)
			if len(stat.SampleValues) > fieldSampleValues {
				stat.SampleValues = stat.SampleValues[:fieldSampleValues]
			}
		}
		out = append(out, stat)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func jsonType(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case float64, int64, int:
		return "number"
	case bool:
		return "boolean"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	}
	return "string"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "") + "…"
}
