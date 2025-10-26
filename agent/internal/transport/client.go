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

func (c *Client) SendReport(report SystemReport, agentID string) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/report/%s", c.coreURL, agentID), bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
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

func (c *Client) RegisterAgent(req AgentRegistrationRequest) (*AgentRegistrationResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal registration request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/register", c.coreURL), bytes.NewBuffer(payload))
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

	logging.Infof("agent registered successfully with ID: %d", regResp.Data.AgentID)
	return &regResp, nil
}
