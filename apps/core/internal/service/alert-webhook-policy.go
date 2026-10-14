package service

import (
	"errors"
	"fmt"
	"net/http"
	"orion/core/internal/config"
	"strings"
	"time"
)

// NewAlertWebhookPolicy returns the SSRF policy for outbound alert webhooks. It
// reuses the Core monitor target policy: link-local, multicast and cloud
// metadata addresses are always blocked; loopback and private ranges are blocked
// unless ORION_ALERT_ALLOW_PRIVATE_WEBHOOKS is set (for self-hosted receivers).
func NewAlertWebhookPolicy(cfg *config.Config) CoreMonitorTargetPolicy {
	return CoreMonitorTargetPolicy{AllowPrivateTargets: cfg != nil && cfg.AlertAllowPrivateWebhooks}
}

// ValidateAlertWebhookURL checks a webhook URL when a channel is created or updated.
func ValidateAlertWebhookURL(cfg *config.Config, rawURL string) error {
	err := NewAlertWebhookPolicy(cfg).ValidateURL(rawURL, "webhook_url")
	if err == nil {
		return nil
	}
	return errors.New(strings.TrimPrefix(err.Error(), ErrCoreManagedMonitorValidation.Error()+": "))
}

// newAlertWebhookClient builds the HTTP client for alert delivery. Addresses are
// re-checked at dial time (so DNS rebinding cannot reach a blocked range) and
// redirects are never followed: a webhook that answers 3xx fails the delivery
// instead of bouncing the signed payload to another host.
func newAlertWebhookClient(cfg *config.Config) *http.Client {
	base := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return NewAlertWebhookPolicy(cfg).HTTPClient(base)
}

var errAlertWebhookRedirect = fmt.Errorf("webhook responded with a redirect, which is not followed")
