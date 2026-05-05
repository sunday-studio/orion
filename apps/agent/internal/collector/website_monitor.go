package collector

import (
	"io"
	"net/http"
	"orion/agent/internal/config"
	"time"
)

type WebsiteMonitorResult struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
	Error     *MonitorError          `json:"error,omitempty"`
}

func RunWebsiteMonitor(cfg config.WebsiteMonitorConfig) *WebsiteMonitorResult {
	start := time.Now()

	timeout := 10 * time.Second
	if cfg.Timeout != "" {
		timeout, _ = time.ParseDuration(cfg.Timeout)
	}

	client := &http.Client{Timeout: timeout}

	req, err := http.NewRequest(http.MethodGet, cfg.URL, nil)
	if err != nil {
		return websiteFailure(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return websiteFailure(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	latency := time.Since(start).Milliseconds()

	status := "up"
	if cfg.ExpectedStatus > 0 && resp.StatusCode != cfg.ExpectedStatus {
		status = "down"
	}

	return &WebsiteMonitorResult{
		Status:    status,
		Timestamp: time.Now().UTC(),
		Metrics: map[string]interface{}{
			"status_code":         resp.StatusCode,
			"latency_ms":          latency,
			"response_size_bytes": len(body),
		},
	}
}

func websiteFailure(err error) *WebsiteMonitorResult {
	return &WebsiteMonitorResult{
		Status:    "down",
		Timestamp: time.Now().UTC(),
		Error: &MonitorError{
			Message: err.Error(),
		},
	}
}
