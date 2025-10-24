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
    coreURL     string
    httpClient  *http.Client
    authToken   string
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

func (c *Client) SendReport(report SystemReport) error {
    payload, err := json.Marshal(report)
    if err != nil {
        return fmt.Errorf("failed to marshal report: %w", err)
    }

    req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/agent/report", c.coreURL), bytes.NewBuffer(payload))
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
