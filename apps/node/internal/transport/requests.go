package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// PulledRequest is something a person asked for in the Console about this host.
type PulledRequest struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

// PullRequests fetches queued Console requests
// (POST /v1/nodes/{node_id}/discovery/requests/pull, node token).
func (c *Client) PullRequests(nodeID string) ([]PulledRequest, error) {
	resp, err := c.makeProtectedRequest("POST", fmt.Sprintf("/v1/nodes/%s/discovery/requests/pull", nodeID), struct{}{})
	if err != nil {
		return nil, fmt.Errorf("failed to pull requests: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusOK:
	case isAuthStatus(resp.StatusCode):
		return nil, NewAuthError(resp.StatusCode, body)
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed:
		return nil, ErrDiscoveryUnsupported
	default:
		return nil, &StatusError{StatusCode: resp.StatusCode, Operation: "request pull"}
	}
	var out struct {
		Data struct {
			Requests []PulledRequest `json:"requests"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse requests: %w", err)
	}
	return out.Data.Requests, nil
}

// SendRequestResult says how a request went. status is applied, failed or
// unsupported; result is the text the Console shows, in the node's own words.
func (c *Client) SendRequestResult(nodeID, requestID, status, result string) error {
	return c.postJSON(fmt.Sprintf("/v1/nodes/%s/discovery/requests/%s/result", nodeID, requestID),
		map[string]string{"status": status, "result": result}, "request result")
}

// DefinitionResult is whether the node could run one Console check.
type DefinitionResult struct {
	DefinitionID string `json:"definition_id"`
	OK           bool   `json:"ok"`
	Reason       string `json:"reason,omitempty"`
}

// SendDefinitionResults reports which Console checks this node cannot run
// (POST /v1/nodes/{node_id}/monitor-definitions/results).
func (c *Client) SendDefinitionResults(nodeID string, results []DefinitionResult) error {
	return c.postJSON(fmt.Sprintf("/v1/nodes/%s/monitor-definitions/results", nodeID),
		map[string]any{"results": results}, "definition results")
}

func (c *Client) postJSON(path string, payload any, op string) error {
	resp, err := c.makeProtectedRequest("POST", path, payload)
	if err != nil {
		return fmt.Errorf("failed to send %s: %w", op, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case isAuthStatus(resp.StatusCode):
		return NewAuthError(resp.StatusCode, body)
	}
	return &StatusError{StatusCode: resp.StatusCode, Operation: op}
}
