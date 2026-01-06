package collector

import (
	"io"
	"net/http"
	"time"
)

type HTTPMonitorConfig struct {
	URL            string
	Timeout        time.Duration
	ExpectedStatus int
}

type HTTPMonitorResult struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics"`
}

func RunHTTPMonitor(cfg HTTPMonitorConfig) (*HTTPMonitorResult, error) {
	start := time.Now()

	client := &http.Client{
		Timeout: cfg.Timeout,
	}

	req, err := http.NewRequest(http.MethodGet, cfg.URL, nil)
	if err != nil {
		return httpMonitorFailureResult(err), nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return httpMonitorFailureResult(err), nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB cap
	latency := time.Since(start).Milliseconds()

	status := "up"
	if cfg.ExpectedStatus > 0 && resp.StatusCode != cfg.ExpectedStatus {
		status = "down"
	}

	return &HTTPMonitorResult{
		Status:    status,
		Timestamp: time.Now().UTC(),
		Metrics: map[string]interface{}{
			"status_code":         resp.StatusCode,
			"latency_ms":          latency,
			"response_size_bytes": len(body),
		},
	}, nil
}

func httpMonitorFailureResult(err error) *HTTPMonitorResult {
	return &HTTPMonitorResult{
		Status:    "down",
		Timestamp: time.Now().UTC(),
		Metrics: map[string]interface{}{
			"error": err.Error(),
		},
	}
}
