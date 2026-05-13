package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration from environment.
type Config struct {
	DataDir                    string
	Port                       string
	AdminUsername              string
	AdminPassword              string
	JWTSecret                  string
	FrontendAuthOn             bool
	AlertChannels              []AlertChannelConfig
	AlertCooldownSeconds       int
	AlertRecoveryNotifications bool
	AlertTLSExpiryDays         int
}

type AlertChannelConfig struct {
	Name         string
	Type         string
	Enabled      bool
	WebhookURL   string
	EmailTo      string
	EmailFrom    string
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
}

// Load reads configuration from environment variables.
// ORION_ADMIN_USERNAME and ORION_ADMIN_PASSWORD must both be set to enable frontend auth.
// ORION_JWT_SECRET is required when frontend auth is enabled.
func Load() *Config {
	adminUser := os.Getenv("ORION_ADMIN_USERNAME")
	adminPass := os.Getenv("ORION_ADMIN_PASSWORD")
	jwtSecret := os.Getenv("ORION_JWT_SECRET")

	frontendAuthOn := adminUser != "" && adminPass != ""

	return &Config{
		DataDir:                    getEnv("ORION_DATA_DIR", "data"),
		Port:                       getEnv("ORION_PORT", "8999"),
		AdminUsername:              adminUser,
		AdminPassword:              adminPass,
		JWTSecret:                  jwtSecret,
		FrontendAuthOn:             frontendAuthOn,
		AlertChannels:              loadAlertChannels(),
		AlertCooldownSeconds:       getEnvInt("ORION_ALERT_COOLDOWN_SECONDS", 300),
		AlertRecoveryNotifications: getEnvBool("ORION_ALERT_RECOVERY_NOTIFICATIONS", true),
		AlertTLSExpiryDays:         getEnvInt("ORION_ALERT_TLS_EXPIRY_DAYS", 14),
	}
}

// Validate returns an error if config is invalid (e.g. frontend auth on but JWT_SECRET empty).
func (c *Config) Validate() error {
	if c.FrontendAuthOn && c.JWTSecret == "" {
		return &ValidationError{Msg: "ORION_JWT_SECRET is required when ORION_ADMIN_USERNAME and ORION_ADMIN_PASSWORD are set"}
	}
	if c.AlertCooldownSeconds < 0 {
		return &ValidationError{Msg: "ORION_ALERT_COOLDOWN_SECONDS must be >= 0"}
	}
	if c.AlertTLSExpiryDays < 0 {
		return &ValidationError{Msg: "ORION_ALERT_TLS_EXPIRY_DAYS must be >= 0"}
	}
	for _, channel := range c.AlertChannels {
		if err := channel.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (c AlertChannelConfig) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return &ValidationError{Msg: "alert channel name is required"}
	}
	switch c.Type {
	case "webhook":
		if strings.TrimSpace(c.WebhookURL) == "" {
			return &ValidationError{Msg: "webhook alert channel requires ORION_ALERT_WEBHOOK_URL"}
		}
	case "email":
		if strings.TrimSpace(c.EmailTo) == "" || strings.TrimSpace(c.EmailFrom) == "" || strings.TrimSpace(c.SMTPHost) == "" || c.SMTPPort <= 0 {
			return &ValidationError{Msg: "email alert channel requires ORION_ALERT_EMAIL_TO, ORION_ALERT_EMAIL_FROM, ORION_ALERT_SMTP_HOST, and ORION_ALERT_SMTP_PORT"}
		}
	default:
		return &ValidationError{Msg: fmt.Sprintf("unsupported alert channel type: %s", c.Type)}
	}
	return nil
}

// ValidationError is returned when config validation fails.
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func loadAlertChannels() []AlertChannelConfig {
	var channels []AlertChannelConfig

	if webhookURL := strings.TrimSpace(os.Getenv("ORION_ALERT_WEBHOOK_URL")); webhookURL != "" {
		channels = append(channels, AlertChannelConfig{
			Name:       getEnv("ORION_ALERT_WEBHOOK_NAME", "default-webhook"),
			Type:       "webhook",
			Enabled:    getEnvBool("ORION_ALERT_WEBHOOK_ENABLED", true),
			WebhookURL: webhookURL,
		})
	}

	if emailTo := strings.TrimSpace(os.Getenv("ORION_ALERT_EMAIL_TO")); emailTo != "" {
		channels = append(channels, AlertChannelConfig{
			Name:         getEnv("ORION_ALERT_EMAIL_NAME", "default-email"),
			Type:         "email",
			Enabled:      getEnvBool("ORION_ALERT_EMAIL_ENABLED", true),
			EmailTo:      emailTo,
			EmailFrom:    os.Getenv("ORION_ALERT_EMAIL_FROM"),
			SMTPHost:     os.Getenv("ORION_ALERT_SMTP_HOST"),
			SMTPPort:     getEnvInt("ORION_ALERT_SMTP_PORT", 587),
			SMTPUsername: os.Getenv("ORION_ALERT_SMTP_USERNAME"),
			SMTPPassword: os.Getenv("ORION_ALERT_SMTP_PASSWORD"),
		})
	}

	return channels
}
