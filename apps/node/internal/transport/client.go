package transport

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"orion/node/internal/logging"
)

// authState holds the bearer token. It is shared by every copy of a Client (see
// WithContext) and guarded so token refresh can race with in-flight requests safely.
type authState struct {
	mu    sync.RWMutex
	token string
}

func (a *authState) get() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.token
}

func (a *authState) set(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.token = token
}

type Client struct {
	coreURL    string
	httpClient *http.Client
	auth       *authState
	ctx        context.Context
	enrollment string
	gzipBodies bool
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
		auth: &authState{token: authToken},
		retry: RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   200 * time.Millisecond,
			MaxDelay:    5 * time.Second,
			JitterRatio: 0.2,
		},
		sleep: time.Sleep,
	}
}

// SetEnrollmentToken sets the shared secret sent on first registration. Core only
// requires it when ORION_ENROLLMENT_TOKEN is configured.
func (c *Client) SetEnrollmentToken(token string) {
	c.enrollment = strings.TrimSpace(token)
}

func (c *Client) SetAuthToken(authToken string) {
	c.auth.set(authToken)
}

// WithContext returns a client that shares this one's connection pool, retry
// settings and auth token but whose requests (and retry waits) are cancelled with ctx.
func (c *Client) WithContext(ctx context.Context) *Client {
	clone := *c
	clone.ctx = ctx
	return &clone
}

func (c *Client) requestContext() context.Context {
	if c.ctx != nil {
		return c.ctx
	}
	return context.Background()
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

	if c.gzipBodies && len(payload) > 1024 {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		if _, err := zw.Write(payload); err == nil && zw.Close() == nil {
			payload = buf.Bytes()
			headers = copyHeaders(headers)
			headers["Content-Encoding"] = "gzip"
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
		if err := c.requestContext().Err(); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(c.requestContext(), method, url, bytes.NewReader(payload))
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

// isAuthStatus is true only for 401, which is what Core answers for a bad or
// revoked node token. A 403 comes from something in front of Core (proxy, WAF,
// captive portal) and is treated as a transient failure, not a credential rejection.
func isAuthStatus(statusCode int) bool {
	return statusCode == http.StatusUnauthorized
}

// StatusError is a non-success HTTP status from Core that is not an auth failure.
type StatusError struct {
	StatusCode int
	Operation  string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("core server returned status %d during %s", e.StatusCode, e.Operation)
}

// IsTransientError reports whether retrying later could succeed: network errors,
// timeouts, 429, 5xx and 403-from-a-proxy. Credential rejections and other 4xx
// responses will not fix themselves.
func IsTransientError(err error) bool {
	if err == nil || IsAuthError(err) || errors.Is(err, ErrEnrollmentRejected) {
		return false
	}
	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		code := statusErr.StatusCode
		return code >= 500 || code == http.StatusTooManyRequests || code == http.StatusForbidden || code == http.StatusRequestTimeout
	}
	return true
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
		"Authorization": "Bearer " + c.auth.get(),
	}
	return c.makeRequest(method, endpoint, body, headers)
}

func (c *Client) RegisterMonitor(req MonitorRegistrationRequest) (*MonitorRegistrationResponse, error) {
	endpoint := fmt.Sprintf("/v1/nodes/%s/register-monitor", req.NodeID)

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
			return nil, NewAuthError(resp.StatusCode, body)
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
	endpoint := fmt.Sprintf("/v1/nodes/%s/unregister-monitor", req.NodeID)

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
			return nil, NewAuthError(resp.StatusCode, body)
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

func (c *Client) SendReport(report SystemReport, nodeID string) error {
	endpoint := fmt.Sprintf("/v1/nodes/%s/report", nodeID)
	resp, err := c.makeProtectedRequest("POST", endpoint, report)
	if err != nil {
		return fmt.Errorf("failed to send report: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if isAuthStatus(resp.StatusCode) {
			return NewAuthError(resp.StatusCode, body)
		}
		logging.Warnf("unexpected status from core: %d — %s", resp.StatusCode, string(body))
		return fmt.Errorf("core server returned status %d", resp.StatusCode)
	}

	logging.Infof("report successfully sent to core")
	return nil
}

func (c *Client) SendMonitorReport(report MonitorReport, nodeID string, monitorID string) error {
	endpoint := fmt.Sprintf("/v1/nodes/%s/%s/report", nodeID, monitorID)
	resp, err := c.makeProtectedRequest("POST", endpoint, report)
	if err != nil {
		return fmt.Errorf("failed to send monitor report: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if isAuthStatus(resp.StatusCode) {
			return NewAuthError(resp.StatusCode, body)
		}
		logging.Warnf("unexpected status from core: %d — %s", resp.StatusCode, string(body))
		return fmt.Errorf("core server returned status %d", resp.StatusCode)
	}

	logging.Infof("monitor report successfully sent to core")
	return nil
}

func (c *Client) SendServiceLogs(batch ServiceLogBatch, nodeID string) error {
	if len(batch.Entries) == 0 {
		return nil
	}

	endpoint := fmt.Sprintf("/v1/nodes/%s/logs/batch", nodeID)
	resp, err := c.makeProtectedRequest("POST", endpoint, batch)
	if err != nil {
		return fmt.Errorf("failed to send service logs: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if isAuthStatus(resp.StatusCode) {
			return NewAuthError(resp.StatusCode, body)
		}
		logging.Warnf("unexpected status from core service log ingest: %d — %s", resp.StatusCode, string(body))
		return fmt.Errorf("core server returned status %d", resp.StatusCode)
	}

	logging.Infof("service logs successfully sent to core")
	return nil
}

// ErrEnrollmentRejected means Core refused the enrollment token; retrying cannot help.
var ErrEnrollmentRejected = errors.New("enrollment token was rejected")

// SetCompression enables gzip for request bodies over 1 KiB ("gzip").
func (c *Client) SetCompression(mode string) { c.gzipBodies = strings.EqualFold(mode, "gzip") }

func (c *Client) RegisterNode(req NodeRegistrationRequest) (*NodeRegistrationResponse, error) {
	var headers map[string]string
	if c.enrollment != "" {
		headers = map[string]string{"X-Orion-Enrollment-Token": c.enrollment}
	}
	resp, err := c.makeRequest("POST", "/v1/register", req, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to send registration request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read registration response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("%w (HTTP %d): create a new one in Console under Add server", ErrEnrollmentRejected, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		logging.Warnf("unexpected status from core during registration: %d — %s", resp.StatusCode, string(body))
		return nil, &StatusError{StatusCode: resp.StatusCode, Operation: "registration"}
	}

	var regResp NodeRegistrationResponse
	if err := json.Unmarshal(body, &regResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal registration response: %w", err)
	}

	if !regResp.Success {
		return nil, fmt.Errorf("registration failed: %s", regResp.Message)
	}

	logging.Infof("node registered successfully with ID: %s", regResp.Data.NodeID)
	return &regResp, nil
}

// SetMaintenanceMode toggles maintenance. When enabling, duration > 0 sets how long
// it lasts (Core expires it automatically; zero uses Core's default of 24h).
func (c *Client) SetMaintenanceMode(nodeID string, maintenanceMode bool, duration time.Duration) error {
	endpoint := fmt.Sprintf("/v1/nodes/%s/maintenance", nodeID)

	payload := map[string]any{
		"maintenance_mode": maintenanceMode,
	}
	if maintenanceMode && duration > 0 {
		payload["duration_seconds"] = int(duration.Seconds())
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

func copyHeaders(h map[string]string) map[string]string {
	out := make(map[string]string, len(h)+1)
	for k, v := range h {
		out[k] = v
	}
	return out
}
