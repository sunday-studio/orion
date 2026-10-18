package logstore

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

// Pattern is a group of similar messages: numbers, ids and addresses are replaced by placeholders.
type Pattern struct {
	Template  string    `json:"pattern"`
	Severity  string    `json:"severity"`
	Count     int64     `json:"count"`
	Trend     []int64   `json:"trend"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	// New is true when the pattern did not appear in the window before this one.
	New      bool   `json:"is_new"`
	SampleID string `json:"sample_id"`
}

// PatternInput is the part of an entry clustering needs.
type PatternInput struct {
	ID       string
	Text     string
	Severity string
	SevNum   int
	At       time.Time
}

var templateRules = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`), "<uuid>"},
	{regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}(?::\d+)?\b`), "<ip>"},
	{regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.-]+`), "<email>"},
	{regexp.MustCompile(`\b[0-9a-fA-F]{12,}\b`), "<hex>"},
	{regexp.MustCompile(`"[^"]{1,80}"|'[^']{1,80}'`), "<str>"},
	{regexp.MustCompile(`\b[a-z]{1,6}_[0-9a-zA-Z]{3,}\b`), "<id>"},
	{regexp.MustCompile(`\d+(?:[.,]\d+)*`), "<n>"},
}

// Template reduces a message to its pattern.
func Template(message string) string {
	line := strings.TrimSpace(message)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	if len(line) > 300 {
		line = line[:300]
	}
	for _, rule := range templateRules {
		line = rule.re.ReplaceAllString(line, rule.repl)
	}
	return strings.Join(strings.Fields(line), " ")
}

// Cluster groups inputs by template over [from, to] and compares with the previous window's templates.
func Cluster(current, previous []PatternInput, from, to time.Time, buckets, limit int) []Pattern {
	if buckets < 2 {
		buckets = 12
	}
	seenBefore := map[string]bool{}
	for _, in := range previous {
		seenBefore[Template(in.Text)] = true
	}
	span := to.Sub(from)
	if span <= 0 {
		span = time.Hour
	}
	type acc struct {
		Pattern
		maxSev int
	}
	byTemplate := map[string]*acc{}
	for _, in := range current {
		key := Template(in.Text)
		if key == "" {
			continue
		}
		a := byTemplate[key]
		if a == nil {
			a = &acc{Pattern: Pattern{Template: key, Trend: make([]int64, buckets), FirstSeen: in.At, LastSeen: in.At, SampleID: in.ID, Severity: in.Severity}, maxSev: in.SevNum}
			byTemplate[key] = a
		}
		a.Count++
		if in.At.Before(a.FirstSeen) {
			a.FirstSeen = in.At
		}
		if in.At.After(a.LastSeen) {
			a.LastSeen, a.SampleID = in.At, in.ID
		}
		if in.SevNum > a.maxSev {
			a.maxSev, a.Severity = in.SevNum, in.Severity
		}
		idx := int(float64(in.At.Sub(from)) / float64(span) * float64(buckets))
		a.Trend[min(max(idx, 0), buckets-1)]++
	}
	out := make([]Pattern, 0, len(byTemplate))
	for key, a := range byTemplate {
		a.New = len(previous) > 0 && !seenBefore[key]
		out = append(out, a.Pattern)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Template < out[j].Template
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// PatternsScanLimit bounds how many entries one pattern request reads per window.
const PatternsScanLimit = 20000

// Patterns clusters the matching entries of [q.From, q.To] and flags patterns absent from the previous window.
func (s *Store) Patterns(q Query, limit int) ([]Pattern, error) {
	q.Cursor = ""
	if q.To.IsZero() {
		q.To = time.Now().UTC()
	}
	if q.From.IsZero() {
		q.From = q.To.Add(-24 * time.Hour)
	}
	read := func(qq Query) ([]PatternInput, error) {
		where, args, err := qq.where()
		if err != nil {
			return nil, err
		}
		var rows []struct {
			ID             string
			Ts             int64
			Severity       string
			SeverityNumber int
			Event          string
			Body           string
		}
		if err := s.dbq(qq).Raw("SELECT id, ts, severity, severity_number, event, body FROM log_entries"+where+" ORDER BY ts DESC LIMIT ?", append(args, PatternsScanLimit)...).Scan(&rows).Error; err != nil {
			return nil, err
		}
		out := make([]PatternInput, 0, len(rows))
		for _, r := range rows {
			text := r.Body
			if r.Event != "" && (text == "" || strings.HasPrefix(text, "{")) {
				text = r.Event
			} else if r.Event != "" {
				text = r.Event + ": " + text
			}
			out = append(out, PatternInput{ID: r.ID, Text: text, Severity: r.Severity, SevNum: r.SeverityNumber, At: time.Unix(0, r.Ts).UTC()})
		}
		return out, nil
	}
	current, err := read(q)
	if err != nil {
		return nil, err
	}
	prev := q
	span := q.To.Sub(q.From)
	prev.From, prev.To = q.From.Add(-span), q.From
	previous, err := read(prev)
	if err != nil {
		return nil, err
	}
	return Cluster(current, previous, q.From, q.To, 12, limit), nil
}
