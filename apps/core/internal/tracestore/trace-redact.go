package tracestore

// Redact applies the app's redaction rules to the free-form parts of every span in place: attributes,
// resource, the attributes of events and links, and the status message. Names, ids and timings are
// kept as sent, so apps should keep secrets out of span names.
func Redact(spans []Span, r Redactor) {
	if r == nil {
		return
	}
	for i := range spans {
		sp := &spans[i]
		sp.Attributes = redactMap(r, sp.Attributes)
		sp.Resource = redactMap(r, sp.Resource)
		if sp.StatusMessage != "" {
			if text, ok := r.Redact(sp.StatusMessage).(string); ok {
				sp.StatusMessage = text
			}
		}
		if len(sp.Events) > 0 {
			events := make([]Event, len(sp.Events))
			for j, e := range sp.Events {
				e.Attributes = redactMap(r, e.Attributes)
				events[j] = e
			}
			sp.Events = events
		}
		if len(sp.Links) > 0 {
			links := make([]Link, len(sp.Links))
			for j, l := range sp.Links {
				l.Attributes = redactMap(r, l.Attributes)
				links[j] = l
			}
			sp.Links = links
		}
	}
}

func redactMap(r Redactor, m map[string]any) map[string]any {
	if len(m) == 0 {
		return m
	}
	if out, ok := r.Redact(m).(map[string]any); ok {
		return out
	}
	return m
}
