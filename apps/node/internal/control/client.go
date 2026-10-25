package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

// ErrUnavailable means no daemon is reachable on the socket.
var ErrUnavailable = errors.New("node control socket is not reachable")

// ErrPermission means the socket exists but the caller may not use it.
var ErrPermission = errors.New("permission denied on node control socket")

// Client talks to the daemon over its unix socket.
type Client struct {
	path string
	http *http.Client
}

// NewClient returns a client for the socket at path ("" uses the default).
func NewClient(path string) *Client {
	if path == "" {
		path = DefaultSocketPath()
	}
	return &Client{
		path: path,
		http: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", path)
				},
			},
		},
	}
}

// Path returns the socket path in use.
func (c *Client) Path() string { return c.path }

func (c *Client) do(method, route string, body any) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, "http://orion-node"+route, rd)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		switch {
		case errors.Is(err, os.ErrPermission):
			return nil, fmt.Errorf("%w: %s (join the %q group or run with sudo)", ErrPermission, c.path, Group)
		case errors.Is(err, os.ErrNotExist), errors.Is(err, net.ErrClosed), isConnRefused(err):
			return nil, fmt.Errorf("%w: %s", ErrUnavailable, c.path)
		}
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		var e Error
		if json.Unmarshal(data, &e) == nil && e.Message != "" {
			return nil, errors.New(e.Message)
		}
		return nil, fmt.Errorf("control request failed: %s", resp.Status)
	}
	return data, nil
}

func isConnRefused(err error) bool {
	var op *net.OpError
	return errors.As(err, &op) && op.Op == "dial"
}

// Status fetches the daemon's status.
func (c *Client) Status() (Status, error) {
	var st Status
	data, err := c.do(http.MethodGet, "/v1/status", nil)
	if err != nil {
		return st, err
	}
	return st, json.Unmarshal(data, &st)
}

// SetMaintenance enables or disables maintenance mode.
func (c *Client) SetMaintenance(enabled bool, reason string, d time.Duration) error {
	_, err := c.do(http.MethodPost, "/v1/maintenance", MaintenanceRequest{Enabled: enabled, Reason: reason, DurationSeconds: int64(d.Seconds())})
	return err
}

// ApplyToken sends a replacement token to the daemon.
func (c *Client) ApplyToken(token string) error {
	_, err := c.do(http.MethodPost, "/v1/token", TokenRequest{Token: token})
	return err
}

// Reload asks the daemon to reload its configuration.
func (c *Client) Reload() error {
	_, err := c.do(http.MethodPost, "/v1/reload", nil)
	return err
}

// Logs returns the tail of the daemon's JSONL log.
func (c *Client) Logs() ([]byte, error) {
	return c.do(http.MethodGet, "/v1/logs", nil)
}

// TokenCheck makes the daemon probe Core with its token and returns the HTTP status.
func (c *Client) TokenCheck() (int, error) {
	data, err := c.do(http.MethodGet, "/v1/token-check", nil)
	if err != nil {
		return 0, err
	}
	var tc TokenCheck
	if err := json.Unmarshal(data, &tc); err != nil {
		return 0, err
	}
	return tc.HTTPStatus, nil
}

// Spool returns the newest n shipped-log entries as NDJSON.
func (c *Client) Spool(n int) ([]byte, error) {
	return c.do(http.MethodGet, "/v1/spool?lines="+strconv.Itoa(n), nil)
}

// Discovery fetches the latest scan.
func (c *Client) Discovery() (Discovery, error) {
	var d Discovery
	data, err := c.do(http.MethodGet, "/v1/discovery", nil)
	if err != nil {
		return d, err
	}
	return d, json.Unmarshal(data, &d)
}

// Rescan runs a scan now and returns it.
func (c *Client) Rescan() (Discovery, error) {
	var d Discovery
	data, err := c.do(http.MethodPost, "/v1/discovery/rescan", nil)
	if err != nil {
		return d, err
	}
	return d, json.Unmarshal(data, &d)
}

// DiscoveryAction dismisses, restores or reviews suggestions.
func (c *Client) DiscoveryAction(action, id string) error {
	_, err := c.do(http.MethodPost, "/v1/discovery/action", DiscoveryAction{Action: action, ID: id})
	return err
}
