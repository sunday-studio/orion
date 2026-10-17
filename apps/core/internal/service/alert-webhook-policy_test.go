package service

import (
	"net"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAlertWebhookClientBlocksPrivateTargetsAtDialTime(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	t.Cleanup(target.Close)

	client := newAlertWebhookClient(&config.Config{})
	_, err := client.Post(target.URL, "application/json", strings.NewReader("{}"))
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("Post() error = %v, want blocked loopback dial", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("blocked target received %d requests, want 0", hits.Load())
	}

	allowed := newAlertWebhookClient(&config.Config{AlertAllowPrivateWebhooks: true})
	resp, err := allowed.Post(target.URL, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Post() with opt-in error = %v", err)
	}
	resp.Body.Close()
	if hits.Load() != 1 {
		t.Fatalf("allowed target hits = %d, want 1", hits.Load())
	}
}

func TestAlertWebhookClientDoesNotFollowRedirects(t *testing.T) {
	var leaked atomic.Int32
	secondHop := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { leaked.Add(1) }))
	t.Cleanup(secondHop.Close)
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, secondHop.URL, http.StatusFound)
	}))
	t.Cleanup(redirector.Close)

	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "hook", Type: "webhook", Enabled: true, WebhookURL: redirector.URL})
	svc := NewAlertService(database, utils.NewLogger(), &config.Config{AlertAllowPrivateWebhooks: true})
	if err := queueAndDispatch(svc, "incident-1", "incident_opened"); err != nil {
		t.Fatal(err)
	}

	if leaked.Load() != 0 {
		t.Fatalf("redirect target received %d requests, want 0", leaked.Load())
	}
	var delivery db.AlertDelivery
	database.First(&delivery)
	if delivery.Status != "failed" || !strings.Contains(delivery.Error, "redirect") {
		t.Fatalf("delivery = status %q error %q, want failed with redirect error", delivery.Status, delivery.Error)
	}
}

func TestAlertWebhookBlocksMetadataEvenWithPrivateOptIn(t *testing.T) {
	policy := NewAlertWebhookPolicy(&config.Config{AlertAllowPrivateWebhooks: true})
	if _, err := policy.DialContext(t.Context(), "tcp", net.JoinHostPort("169.254.169.254", "80")); err == nil {
		t.Fatal("DialContext() to metadata address succeeded, want blocked")
	}
}

func TestWebhookPolicyIgnoresProxyEnvironment(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://10.255.255.1:3128")
	t.Setenv("HTTP_PROXY", "http://10.255.255.1:3128")
	client := newAlertWebhookClient(&config.Config{})
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil {
		t.Fatal("the policy transport must not honour proxy variables: the dial check would validate the proxy, not the target")
	}
}
