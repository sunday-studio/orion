package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration from environment.
type Config struct {
	DataDir                    string
	Port                       string
	CORSOrigins                []string
	AdminUsername              string
	AdminPassword              string
	JWTSecret                  string
	FrontendAuthOn             bool
	LoginRateLimitAttempts     int
	LoginRateLimitWindowSecs   int
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
		CORSOrigins:                getEnvList("ORION_CORS_ORIGINS", []string{"http://localhost:5173", "http://127.0.0.1:5173"}),
		AdminUsername:              adminUser,
		AdminPassword:              adminPass,
		JWTSecret:                  jwtSecret,
		FrontendAuthOn:             frontendAuthOn,
		LoginRateLimitAttempts:     getEnvInt("ORION_LOGIN_RATE_LIMIT_ATTEMPTS", 5),
		LoginRateLimitWindowSecs:   getEnvInt("ORION_LOGIN_RATE_LIMIT_WINDOW_SECONDS", 60),
		AlertChannels:              loadAlertChannels(),
		AlertCooldownSeconds:       getEnvInt("ORION_ALERT_COOLDOWN_SECONDS", 300),
		AlertRecoveryNotifications: getEnvBool("ORION_ALERT_RECOVERY_NOTIFICATIONS", true),
		AlertTLSExpiryDays:         getEnvInt("ORION_ALERT_TLS_EXPIRY_DAYS", 14),
	}
}

// LoadDotEnv loads KEY=VALUE pairs from a dotenv file without overriding existing
// process environment variables.
func LoadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid dotenv line %d: missing '='", lineNumber)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("invalid dotenv line %d: empty key", lineNumber)
		}
		value = parseDotEnvValue(strings.TrimSpace(value))

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s from dotenv: %w", key, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func parseDotEnvValue(value string) string {
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) && len(value) >= 2 {
		if unquoted, err := strconv.Unquote(value); err == nil {
			return unquoted
		}
	}
	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) >= 2 {
		return value[1 : len(value)-1]
	}

	if hashIndex := strings.Index(value, " #"); hashIndex >= 0 {
		value = value[:hashIndex]
	}
	return strings.TrimSpace(value)
}

// Validate returns an error if config is invalid (e.g. frontend auth on but JWT_SECRET empty).
func (c *Config) Validate() error {
	authValues := []string{
		strings.TrimSpace(c.AdminUsername),
		strings.TrimSpace(c.AdminPassword),
		strings.TrimSpace(c.JWTSecret),
	}
	authValueCount := 0
	for _, value := range authValues {
		if value != "" {
			authValueCount++
		}
	}
	if authValueCount > 0 && authValueCount < len(authValues) {
		return &ValidationError{Msg: "ORION_ADMIN_USERNAME, ORION_ADMIN_PASSWORD, and ORION_JWT_SECRET must all be set together"}
	}
	if c.FrontendAuthOn && c.JWTSecret == "" {
		return &ValidationError{Msg: "ORION_JWT_SECRET is required when ORION_ADMIN_USERNAME and ORION_ADMIN_PASSWORD are set"}
	}
	if c.LoginRateLimitAttempts < 0 {
		return &ValidationError{Msg: "ORION_LOGIN_RATE_LIMIT_ATTEMPTS must be >= 0"}
	}
	if c.LoginRateLimitWindowSecs < 0 {
		return &ValidationError{Msg: "ORION_LOGIN_RATE_LIMIT_WINDOW_SECONDS must be >= 0"}
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

func getEnvList(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return fallback
	}
	return items
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
