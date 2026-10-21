package transport

import (
	"fmt"
	"net/http"
)

// PreflightEnrollment verifies a first-registration credential without
// consuming it. Call SetEnrollmentToken before invoking it.
func (c *Client) PreflightEnrollment() error {
	headers := map[string]string{}
	if c.enrollment != "" {
		headers["X-Orion-Enrollment-Token"] = c.enrollment
	}
	resp, err := c.makeRequest(http.MethodPost, "/v1/register/preflight", nil, headers)
	if err != nil {
		return fmt.Errorf("check enrollment token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("%w (HTTP %d): create a fresh token in Console under Servers, Add server", ErrEnrollmentRejected, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return &StatusError{StatusCode: resp.StatusCode, Operation: "enrollment preflight"}
	}
	return nil
}
