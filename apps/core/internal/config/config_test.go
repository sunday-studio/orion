package config

import "testing"

func TestLoadAlertChannelsFromEnvironment(t *testing.T) {
	t.Setenv("ORION_ALERT_WEBHOOK_URL", "https://example.com/hook")
	t.Setenv("ORION_ALERT_EMAIL_TO", "ops@example.com")
	t.Setenv("ORION_ALERT_EMAIL_FROM", "orion@example.com")
	t.Setenv("ORION_ALERT_SMTP_HOST", "smtp.example.com")
	t.Setenv("ORION_ALERT_SMTP_PORT", "2525")

	cfg := Load()

	if len(cfg.AlertChannels) != 2 {
		t.Fatalf("alert channel count = %d, want 2", len(cfg.AlertChannels))
	}
	if cfg.AlertChannels[0].Type != "webhook" || cfg.AlertChannels[0].WebhookURL != "https://example.com/hook" {
		t.Fatalf("webhook channel = %+v", cfg.AlertChannels[0])
	}
	if cfg.AlertChannels[1].Type != "email" || cfg.AlertChannels[1].SMTPPort != 2525 {
		t.Fatalf("email channel = %+v", cfg.AlertChannels[1])
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestLoadCORSOriginsFromEnvironment(t *testing.T) {
	t.Setenv("ORION_CORS_ORIGINS", "https://console.example.com, http://localhost:5173 ")

	cfg := Load()

	if len(cfg.CORSOrigins) != 2 {
		t.Fatalf("CORSOrigins len = %d, want 2", len(cfg.CORSOrigins))
	}
	if cfg.CORSOrigins[0] != "https://console.example.com" || cfg.CORSOrigins[1] != "http://localhost:5173" {
		t.Fatalf("CORSOrigins = %#v", cfg.CORSOrigins)
	}
}

func TestValidateRejectsIncompleteEmailAlertChannel(t *testing.T) {
	cfg := &Config{
		AlertChannels: []AlertChannelConfig{
			{Name: "email", Type: "email", Enabled: true, EmailTo: "ops@example.com"},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want incomplete email channel error")
	}
}
