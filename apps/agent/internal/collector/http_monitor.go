package collector

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type HTTPMonitorConfig struct {
	URL               string
	Timeout           time.Duration
	ExpectedStatus    int
	ExpectedBody      string
	ExpectedBodyRegex string
}

type HTTPMonitorResult struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics"`
}

func RunHTTPMonitor(cfg HTTPMonitorConfig) (*HTTPMonitorResult, error) {
	client := &http.Client{
		Timeout: cfg.Timeout,
	}

	return runHTTPMonitorWithClient(cfg, client)
}

func runHTTPMonitorWithClient(cfg HTTPMonitorConfig, client *http.Client) (*HTTPMonitorResult, error) {
	start := time.Now()

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
	var failureReason string
	if cfg.ExpectedStatus > 0 && resp.StatusCode != cfg.ExpectedStatus {
		status = "down"
		failureReason = "unexpected status code"
	}

	bodyText := string(body)
	if status == "up" && cfg.ExpectedBody != "" && !strings.Contains(bodyText, cfg.ExpectedBody) {
		status = "down"
		failureReason = "expected body content not found"
	}

	if status == "up" && cfg.ExpectedBodyRegex != "" {
		matched, err := regexp.MatchString(cfg.ExpectedBodyRegex, bodyText)
		if err != nil {
			status = "down"
			failureReason = "invalid expected body regex"
		} else if !matched {
			status = "down"
			failureReason = "expected body regex did not match"
		}
	}

	metrics := map[string]interface{}{
		"status_code":         resp.StatusCode,
		"latency_ms":          latency,
		"response_size_bytes": len(body),
	}
	if failureReason != "" {
		metrics["failure_reason"] = failureReason
	}

	return &HTTPMonitorResult{
		Status:    status,
		Timestamp: time.Now().UTC(),
		Metrics:   metrics,
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
