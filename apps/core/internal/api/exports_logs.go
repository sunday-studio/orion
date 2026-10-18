package api

import (
	"context"
	"encoding/json"
	"net/url"
	"orion/core/internal/logstore"
	"time"
)

// logsExportSource streams product logs for the Service logs tab.
func (s *Server) logsExportSource(internal bool) exportSource {
	return exportSource{
		header: []string{"timestamp", "severity", "source", "event", "message", "request_id", "trace_id", "attributes"},
		run: func(ctx context.Context, values url.Values, limit int, emit func([]string, map[string]any) error) error {
			q := logstore.Query{AppIDs: splitList(values.Get("app_id")), Severity: values.Get("severity"), Event: values.Get("event"), Text: values.Get("q"),
				TraceID: values.Get("trace_id"), RequestID: values.Get("request_id"), Attrs: values["attr"], Limit: logstore.MaxPageSize, Ctx: ctx}
			if internal {
				q.AppIDs = []string{logstore.InternalAppID}
			}
			now := time.Now().UTC()
			q.From, _ = parseLogTime(values.Get("from"), now)
			q.To, _ = parseLogTime(values.Get("to"), now)
			sent := 0
			names := map[string]string{}
			for sent < limit {
				entries, next, err := s.logs.store.List(q)
				if err != nil {
					return err
				}
				var missing []string
				for _, e := range entries {
					if _, ok := names[e.AppID]; !ok {
						names[e.AppID] = ""
						missing = append(missing, e.AppID)
					}
				}
				for id, name := range s.appNames(missing) {
					names[id] = name
				}
				for _, e := range entries {
					if sent >= limit {
						break
					}
					attrs, _ := json.Marshal(e.Attributes)
					cells := []string{e.Timestamp.UTC().Format(time.RFC3339Nano), e.Severity, names[e.AppID], e.Event, e.Body, e.RequestID, e.TraceID, string(attrs)}
					full := map[string]any{}
					raw, _ := json.Marshal(toLogEntry(e, names))
					_ = json.Unmarshal(raw, &full)
					if err := emit(cells, full); err != nil {
						return err
					}
					sent++
				}
				if next == "" || len(entries) == 0 {
					return nil
				}
				q.Cursor = next
			}
			return nil
		},
	}
}
