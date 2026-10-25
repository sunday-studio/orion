package transport

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// DiscoveredItem is one thing the node found on its host.
type DiscoveredItem struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Detail     string `json:"detail,omitempty"`
	Confidence string `json:"confidence"`
	Status     string `json:"status"`
	ManagedIn  string `json:"managed_in,omitempty"`
	New        bool   `json:"new,omitempty"`
	Dismissed  bool   `json:"dismissed,omitempty"`
}

// DiscoveredKind says how scanning one kind of thing went.
type DiscoveredKind struct {
	Kind    string `json:"kind"`
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
	Fix     string `json:"fix,omitempty"`
	Why     string `json:"why,omitempty"`
}

// DiscoveryReport is the latest scan, sent to Core after each scan.
type DiscoveryReport struct {
	ScannedAt time.Time        `json:"scanned_at"`
	Enabled   bool             `json:"enabled"`
	Kinds     []DiscoveredKind `json:"kinds"`
	Items     []DiscoveredItem `json:"items"`
	// Capabilities tells Core what this node can do beyond reporting, e.g. "requests"
	// (it polls and applies Console requests). Core reads flags, never versions.
	Capabilities []string `json:"capabilities,omitempty"`
}

// ErrDiscoveryUnsupported means this Core does not take discovery reports yet.
var ErrDiscoveryUnsupported = fmt.Errorf("this Core does not accept discovery reports")

// SendDiscovery reports the latest scan (POST /v1/nodes/{node_id}/discovery, node token).
func (c *Client) SendDiscovery(nodeID string, report DiscoveryReport) error {
	resp, err := c.makeProtectedRequest("POST", fmt.Sprintf("/v1/nodes/%s/discovery", nodeID), report)
	if err != nil {
		return fmt.Errorf("failed to send discovery: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case isAuthStatus(resp.StatusCode):
		return NewAuthError(resp.StatusCode, body)
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed:
		return ErrDiscoveryUnsupported // an older Core: not an error worth repeating
	}
	return &StatusError{StatusCode: resp.StatusCode, Operation: "discovery report"}
}
