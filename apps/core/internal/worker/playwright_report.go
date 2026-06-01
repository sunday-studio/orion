package worker

import (
	"orion/core/internal/service"
	"time"
)

func (a *App) storePlaywrightReport(monitorID string, result playwrightResult) error {
	payload := service.MonitorReportPayload{
		Timestamp: result.FinishedAt.Format(time.RFC3339Nano),
		Health:    result.Health,
		Metrics:   playwrightPayload(result, nil),
	}
	if result.Error != nil {
		payload.Error = playwrightPayload(result, result.Error)
	}
	_, err := a.reports.StoreMonitorReport(monitorID, payload)
	return err
}

func playwrightPayload(result playwrightResult, resultErr error) map[string]any {
	payload := map[string]any{
		"runner":                "core",
		"type":                  "playwright_transaction",
		"target_url":            service.SanitizeCoreMonitorURL(result.TargetURL),
		"browser":               result.Browser,
		"headless":              result.Headless,
		"viewport":              result.Viewport,
		"step_count":            result.StepCount,
		"completed_steps":       result.CompletedSteps,
		"steps":                 result.Steps,
		"artifacts":             result.Artifacts,
		"artifact_bytes":        result.ArtifactBytes,
		"artifact_limit_bytes":  result.ArtifactLimitBytes,
		"screenshot_on_failure": result.ScreenshotOnFailure,
		"redacted_variables":    result.RedactedVariables,
		"redacted_values":       result.RedactedVariables,
		"redacted_headers":      result.RedactedHeaders,
		"duration_ms":           result.Duration.Milliseconds(),
		"ok":                    result.Health == "up",
		"collected_at":          result.FinishedAt.Format(time.RFC3339Nano),
		"failure_stage":         result.FailureStage,
		"failure_step":          result.FailureStep,
		"failure_index":         result.FailureIndex,
	}
	if resultErr != nil {
		payload["error"] = resultErr.Error()
	}
	return payload
}
