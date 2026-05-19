package transport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"orion/agent/internal/logging"
)

type AuthError struct {
	StatusCode int
	Body       string
}

func (e *AuthError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("core authentication failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("core authentication failed with status %d: %s", e.StatusCode, e.Body)
}

func IsAuthError(err error) bool {
	var authErr *AuthError
	return errors.As(err, &authErr)
}

type Client struct {
	coreURL    string
	httpClient *http.Client
	authToken  string
	retry      RetryConfig
	sleep      func(time.Duration)
}

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	JitterRatio float64
}

func NewClient(coreURL, authToken string) *Client {
	return &Client{
		coreURL: coreURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		authToken: authToken,
		retry: RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   200 * time.Millisecond,
			MaxDelay:    5 * time.Second,
			JitterRatio: 0.2,
		},
		sleep: time.Sleep,
	}
}

func (c *Client) SetAuthToken(authToken string) {
	c.authToken = authToken
}

func (c *Client) makeRequest(method, endpoint string, body interface{}, headers map[string]string) (*http.Response, error) {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	logging.Debugf("core request prepared: method=%s endpoint=%s body_bytes=%d", method, endpoint, len(payload))
	return c.sendWithRetry(method, fmt.Sprintf("%s%s", c.coreURL, endpoint), payload, headers)
}

func (c *Client) sendWithRetry(method string, url string, payload []byte, headers map[string]string) (*http.Response, error) {
	attempts := c.retry.MaxAttempts
	if attempts <= 0 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		logging.Debugf("core request attempt: method=%s url=%s attempt=%d/%d", method, url, attempt, attempts)
		req, err := http.NewRequest(method, url, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		resp, err := c.httpClient.Do(req)
		if err == nil && !shouldRetryStatus(resp.StatusCode) {
			logging.Debugf("core request completed: method=%s url=%s status=%d attempt=%d", method, url, resp.StatusCode, attempt)
			return resp, nil
		}

		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("core server returned status %d", resp.StatusCode)
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}

		if attempt == attempts {
			break
		}

		delay := c.retryDelay(attempt)
		logging.Warnf("request failed; retrying in %s (attempt %d/%d): %v", delay, attempt+1, attempts, lastErr)
		c.sleep(delay)
	}

	return nil, fmt.Errorf("failed to send request after %d attempts: %w", attempts, lastErr)
}

func shouldRetryStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}

func isAuthStatus(statusCode int) bool {
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
}

func (c *Client) retryDelay(attempt int) time.Duration {
	baseDelay := c.retry.BaseDelay
	if baseDelay <= 0 {
		baseDelay = 200 * time.Millisecond
	}
	maxDelay := c.retry.MaxDelay
	if maxDelay <= 0 {
		maxDelay = 5 * time.Second
	}

	delay := baseDelay * (1 << (attempt - 1))
	if delay > maxDelay {
		delay = maxDelay
	}

	if c.retry.JitterRatio <= 0 {
		return delay
	}

	jitterRange := int64(float64(delay) * c.retry.JitterRatio)
	if jitterRange <= 0 {
		return delay
	}

	jitter := time.Duration(rand.Int63n(jitterRange*2+1) - jitterRange)
	return delay + jitter
}

func (c *Client) makeProtectedRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + c.authToken,
	}
	return c.makeRequest(method, endpoint, body, headers)
}

func (c *Client) RegisterMonitor(req MonitorRegistrationRequest) (*MonitorRegistrationResponse, error) {
	endpoint := fmt.Sprintf("/v1/agents/%s/register-monitor", req.AgentID)

	resp, err := c.makeProtectedRequest("POST", endpoint, req)
	if err != nil {
		return nil, fmt.Errorf("failed to register monitor: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if isAuthStatus(resp.StatusCode) {
			return nil, &AuthError{StatusCode: resp.StatusCode, Body: string(body)}
		}
		return nil, fmt.Errorf("core responded with %d: %s", resp.StatusCode, string(body))
	}

	var monResp MonitorRegistrationResponse
	if err := json.Unmarshal(body, &monResp); err != nil {
		return nil, fmt.Errorf("failed to parse application registration response: %w", err)
	}

	if !monResp.Success {
		return nil, fmt.Errorf("monitor registration failed: %s", monResp.Message)
	}

	logging.Infof("Monitor registered successfully with ID: %s", monResp.Data.MonitorID)
	return &monResp, nil
}

func (c *Client) UnregisterMonitor(req UnRegisterMonitorRequest) (*UnRegisterMonitorResponse, error) {
	endpoint := fmt.Sprintf("/v1/agents/%s/unregister-monitor", req.AgentID)

	resp, err := c.makeProtectedRequest("POST", endpoint, req)
	if err != nil {
		return nil, fmt.Errorf("failed to unregister monitor: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if isAuthStatus(resp.StatusCode) {
			return nil, &AuthError{StatusCode: resp.StatusCode, Body: string(body)}
		}
		return nil, fmt.Errorf("core responded with %d: %s", resp.StatusCode, string(body))
	}

	var monResp UnRegisterMonitorResponse
	if err := json.Unmarshal(body, &monResp); err != nil {
		return nil, fmt.Errorf("failed to parse application registration response: %w", err)
	}

	if !monResp.Success {
		return nil, fmt.Errorf("monitor registration failed: %s", monResp.Message)
	}

	logging.Infof("Monitor unregistered successfully with ID: %s", req.MonitorID)
	return &monResp, nil
}

func (c *Client) SendReport(report SystemReport, agentID string) error {
	endpoint := fmt.Sprintf("/v1/agents/%s/report", agentID)
	resp, err := c.makeProtectedRequest("POST", endpoint, report)
	if err != nil {
		return fmt.Errorf("failed to send report: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if isAuthStatus(resp.StatusCode) {
			return &AuthError{StatusCode: resp.StatusCode, Body: string(body)}
		}
		logging.Warnf("unexpected status from core: %d — %s", resp.StatusCode, string(body))
		return fmt.Errorf("core server returned status %d", resp.StatusCode)
	}

	logging.Infof("report successfully sent to core")
	return nil
}

func (c *Client) SendMonitorReport(report MonitorReport, agentID string, monitorID string) error {
	endpoint := fmt.Sprintf("/v1/agents/%s/%s/report", agentID, monitorID)
	resp, err := c.makeProtectedRequest("POST", endpoint, report)
	if err != nil {
		return fmt.Errorf("failed to send monitor report: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if isAuthStatus(resp.StatusCode) {
			return &AuthError{StatusCode: resp.StatusCode, Body: string(body)}
		}
		logging.Warnf("unexpected status from core: %d — %s", resp.StatusCode, string(body))
		return fmt.Errorf("core server returned status %d", resp.StatusCode)
	}

	logging.Infof("monitor report successfully sent to core")
	return nil
}

func (c *Client) RegisterAgent(req AgentRegistrationRequest) (*AgentRegistrationResponse, error) {
	resp, err := c.makeRequest("POST", "/v1/register", req, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to send registration request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read registration response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logging.Warnf("unexpected status from core during registration: %d — %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("core server returned status %d during registration", resp.StatusCode)
	}

	var regResp AgentRegistrationResponse
	if err := json.Unmarshal(body, &regResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal registration response: %w", err)
	}

	if !regResp.Success {
		return nil, fmt.Errorf("registration failed: %s", regResp.Message)
	}

	logging.Infof("agent registered successfully with ID: %s", regResp.Data.AgentID)
	return &regResp, nil
}

func (c *Client) SetMaintenanceMode(agentID string, maintenanceMode bool) error {
	endpoint := fmt.Sprintf("/v1/agents/%s/maintenance", agentID)

	payload := map[string]bool{
		"maintenance_mode": maintenanceMode,
	}

	resp, err := c.makeProtectedRequest("PUT", endpoint, payload)
	if err != nil {
		return fmt.Errorf("failed to set maintenance mode: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if isAuthStatus(resp.StatusCode) {
			return &AuthError{StatusCode: resp.StatusCode, Body: string(body)}
		}
		logging.Warnf("unexpected status from core: %d — %s", resp.StatusCode, string(body))
		return fmt.Errorf("core server returned status %d", resp.StatusCode)
	}

	logging.Infof("Maintenance mode updated successfully")
	return nil
}
