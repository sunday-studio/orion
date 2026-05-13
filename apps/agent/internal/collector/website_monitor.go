package collector

import (
	"io"
	"net"
	"net/http"
	"net/url"
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
	timeout := 10 * time.Second
	if cfg.Timeout != "" {
		timeout, _ = time.ParseDuration(cfg.Timeout)
	}

	client := &http.Client{Timeout: timeout}
	return runWebsiteMonitorWithClientAndResolver(cfg, client, net.LookupHost)
}

func runWebsiteMonitorWithClientAndResolver(cfg config.WebsiteMonitorConfig, client *http.Client, lookupHost func(string) ([]string, error)) *WebsiteMonitorResult {
	start := time.Now()

	parsedURL, err := url.Parse(cfg.URL)
	if err != nil {
		return websiteFailure(err)
	}

	resolvedIPs, dnsErr := lookupHost(parsedURL.Hostname())
	if dnsErr != nil {
		return &WebsiteMonitorResult{
			Status:    "down",
			Timestamp: time.Now().UTC(),
			Metrics: map[string]interface{}{
				"failure_reason": "dns resolution failed",
			},
			Error: &MonitorError{
				Message: dnsErr.Error(),
			},
		}
	}

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
	var failureReason string
	if cfg.ExpectedStatus > 0 && resp.StatusCode != cfg.ExpectedStatus {
		status = "down"
		failureReason = "unexpected status code"
	}

	metrics := map[string]interface{}{
		"status_code":         resp.StatusCode,
		"latency_ms":          latency,
		"response_size_bytes": len(body),
	}
	if len(resolvedIPs) > 0 {
		metrics["resolved_ips"] = resolvedIPs
	}
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		expiresAt := resp.TLS.PeerCertificates[0].NotAfter.UTC()
		metrics["tls_expires_at"] = expiresAt.Format(time.RFC3339)
		metrics["tls_days_remaining"] = int(time.Until(expiresAt).Hours() / 24)
	}
	if failureReason != "" {
		metrics["failure_reason"] = failureReason
	}

	return &WebsiteMonitorResult{
		Status:    status,
		Timestamp: time.Now().UTC(),
		Metrics:   metrics,
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
