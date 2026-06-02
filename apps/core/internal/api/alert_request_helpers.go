package api

import (
	"encoding/json"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
)

func encodeStringList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	body, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(body)
}
func mergeAlertRouteContext(loaded service.AlertRouteContext, overrides service.AlertRouteContext) service.AlertRouteContext {
	loaded.EventType = overrides.EventType
	if overrides.Severity != "" {
		loaded.Severity = overrides.Severity
	}
	if overrides.AgentID != "" {
		loaded.AgentID = overrides.AgentID
	}
	if overrides.MonitorID != "" {
		loaded.MonitorID = overrides.MonitorID
	}
	if overrides.MonitorType != "" {
		loaded.MonitorType = overrides.MonitorType
	}
	return loaded
}
func validateAlertEvents(events []string) error {
	for _, event := range events {
		if event = strings.TrimSpace(event); event != "" && !db.ValidAlertEvent(event) {
			return &requestValidationError{message: "unsupported alert channel event"}
		}
	}
	return nil
}

type requestValidationError struct{ message string }

func (e *requestValidationError) Error() string {
	return e.message
}
