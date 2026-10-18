package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// PulledDefinition is a check configured for this agent in the Console.
type PulledDefinition struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	CheckType       string         `json:"check_type"`
	Params          map[string]any `json:"params"`
	IntervalSeconds int            `json:"interval_seconds"`
	FailAfter       int            `json:"fail_after"`
}

// PullMonitorDefinitions fetches the Console-configured checks for this agent
// (POST /v1/agents/{agent_id}/monitor-definitions/sync, agent token).
func (c *Client) PullMonitorDefinitions(agentID string) ([]PulledDefinition, error) {
	endpoint := fmt.Sprintf("/v1/agents/%s/monitor-definitions/sync", agentID)
	resp, err := c.makeProtectedRequest("POST", endpoint, struct{}{})
	if err != nil {
		return nil, fmt.Errorf("failed to sync monitor definitions: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		if isAuthStatus(resp.StatusCode) {
			return nil, NewAuthError(resp.StatusCode, body)
		}
		return nil, &StatusError{StatusCode: resp.StatusCode, Operation: "monitor definition sync"}
	}
	var out struct {
		Data struct {
			Definitions []PulledDefinition `json:"definitions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse monitor definitions: %w", err)
	}
	return out.Data.Definitions, nil
}
