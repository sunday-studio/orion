package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"orion/agent/internal/logging"
)

type Client struct {
	coreURL    string
	httpClient *http.Client
	authToken  string
}

func NewClient(coreURL, authToken string) *Client {
	return &Client{
		coreURL: coreURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		authToken: authToken,
	}
}

func (c *Client) SetAuthToken(authToken string) {
	c.authToken = authToken
}

func (c *Client) makeRequest(method, endpoint string, body interface{}, headers map[string]string) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(method, fmt.Sprintf("%s%s", c.coreURL, endpoint), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return resp, nil
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
		logging.Warnf("unexpected status from core: %d — %s", resp.StatusCode, string(body))
		return fmt.Errorf("core server returned status %d", resp.StatusCode)
	}

	logging.Infof("monitor report successfully sent to core")
	return nil
}

func (c *Client) RegisterAgent(req AgentRegistrationRequest) (*AgentRegistrationResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal registration request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/register", c.coreURL), bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create registration request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
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
		logging.Warnf("unexpected status from core: %d — %s", resp.StatusCode, string(body))
		return fmt.Errorf("core server returned status %d", resp.StatusCode)
	}

	logging.Infof("Maintenance mode updated successfully")
	return nil
}
