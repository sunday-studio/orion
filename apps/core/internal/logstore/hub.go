package logstore

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

// Subscription receives newly written entries that match a filter.
type Subscription struct {
	C       <-chan Entry
	Dropped atomic.Int64 // entries skipped because this consumer was too slow
	ch      chan Entry
	match   func(Entry) bool
}

// hub fans written entries out to live-tail subscribers. A slow subscriber loses
// entries (counted in Dropped) rather than slowing the writer.
type hub struct {
	mu   sync.RWMutex
	subs map[*Subscription]struct{}
}

func newHub() *hub { return &hub{subs: map[*Subscription]struct{}{}} }

// Subscribe registers a live consumer. Call the returned cancel when done.
func (s *Store) Subscribe(q Query) (*Subscription, func()) {
	ch := make(chan Entry, 256)
	sub := &Subscription{C: ch, ch: ch, match: q.Match}
	s.hub.mu.Lock()
	s.hub.subs[sub] = struct{}{}
	s.hub.mu.Unlock()
	return sub, func() {
		s.hub.mu.Lock()
		delete(s.hub.subs, sub)
		s.hub.mu.Unlock()
	}
}

func (h *hub) publish(entries []Entry) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.subs) == 0 {
		return
	}
	for sub := range h.subs {
		for _, e := range entries {
			if !sub.match(e) {
				continue
			}
			select {
			case sub.ch <- e:
			default:
				sub.Dropped.Add(1)
			}
		}
	}
}

// Match reports whether an entry satisfies the filters of the query that can be
// evaluated in memory (everything except paging and the time range).
func (q Query) Match(e Entry) bool {
	if len(q.AppIDs) > 0 {
		found := false
		for _, id := range q.AppIDs {
			if id == e.AppID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	} else if !q.IncludeInternal && e.AppID == InternalAppID {
		return false
	}
	if q.Severity != "" {
		if min, max, ok := SeverityRange(q.Severity); ok {
			if e.SeverityNumber < min || e.SeverityNumber > max {
				return false
			}
		} else {
			found := false
			for _, name := range strings.Split(q.Severity, ",") {
				if canonical, _ := NormalizeSeverity(strings.TrimSpace(name)); canonical == e.Severity {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	if q.Event != "" && e.Event != q.Event {
		return false
	}
	if q.TraceID != "" && e.TraceID != q.TraceID {
		return false
	}
	if q.RequestID != "" && e.RequestID != q.RequestID {
		return false
	}
	if q.Text != "" {
		needle := strings.ToLower(q.Text)
		if !strings.Contains(strings.ToLower(e.Body), needle) && !strings.Contains(strings.ToLower(e.Event), needle) && !strings.Contains(strings.ToLower(encodeJSON(e.Attributes)), needle) {
			return false
		}
	}
	for _, attr := range q.Attrs {
		key, value, _ := strings.Cut(attr, ":")
		got, ok := e.Attributes[key]
		if !ok || fmt.Sprint(got) != value {
			return false
		}
	}
	return true
}

// After returns up to limit entries with an id greater than afterID (oldest first),
// for resuming a live stream with Last-Event-ID.
func (s *Store) After(afterID string, q Query, limit int) ([]Entry, error) {
	q.Cursor = ""
	where, args, err := q.where()
	if err != nil {
		return nil, err
	}
	clause := " WHERE id > ?"
	if where != "" {
		clause = where + " AND id > ?"
	}
	args = append(args, afterID, limit)
	var rows []logRow
	if err := s.db.Raw("SELECT * FROM log_entries"+clause+" ORDER BY id ASC LIMIT ?", args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Entry, len(rows))
	for i, r := range rows {
		out[i] = r.entry()
	}
	return out, nil
}
