package config

import (
	"bufio"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration from environment.
type Config struct {
	DataDir                        string
	Port                           string
	CORSOrigins                    []string
	CoreWorkerID                   string
	CoreWorkerHeartbeatSeconds     int
	CoreWorkerStaleSeconds         int
	CoreMonitorAllowPrivateTargets bool
	DataLifecycleSchedulerSeconds  int
	AdminUsername                  string
	AdminPassword                  string
	JWTSecret                      string
	FrontendAuthOn                 bool
	LoginRateLimitAttempts         int
	LoginRateLimitWindowSecs       int
	AlertCooldownSeconds           int
	AlertRecoveryNotifications     bool
	AlertTLSExpiryDays             int
	PublicStatusMailEnabled        bool
	PublicStatusMailHost           string
	PublicStatusMailPort           int
	PublicStatusMailUsername       string
	PublicStatusMailPassword       string
	PublicStatusMailFromEmail      string
	PublicStatusMailFromName       string
	PublicStatusMailReplyTo        string
	PublicStatusURLOrigin          string
	PublicStatusSubscriberSecret   string
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
		DataDir:                        getEnv("ORION_DATA_DIR", "data"),
		Port:                           getEnv("ORION_PORT", "8999"),
		CORSOrigins:                    getEnvList("ORION_CORS_ORIGINS", []string{"http://localhost:5173", "http://127.0.0.1:5173"}),
		CoreWorkerID:                   getEnv("ORION_WORKER_ID", "core-monitor-worker"),
		CoreWorkerHeartbeatSeconds:     getEnvInt("ORION_WORKER_HEARTBEAT_SECONDS", 15),
		CoreWorkerStaleSeconds:         getEnvInt("ORION_WORKER_STALE_SECONDS", 60),
		CoreMonitorAllowPrivateTargets: getEnvBool("ORION_CORE_MONITOR_ALLOW_PRIVATE_TARGETS", false),
		DataLifecycleSchedulerSeconds:  getEnvInt("ORION_DATA_LIFECYCLE_SCHEDULER_SECONDS", 3600),
		AdminUsername:                  adminUser,
		AdminPassword:                  adminPass,
		JWTSecret:                      jwtSecret,
		FrontendAuthOn:                 frontendAuthOn,
		LoginRateLimitAttempts:         getEnvInt("ORION_LOGIN_RATE_LIMIT_ATTEMPTS", 5),
		LoginRateLimitWindowSecs:       getEnvInt("ORION_LOGIN_RATE_LIMIT_WINDOW_SECONDS", 60),
		AlertCooldownSeconds:           getEnvInt("ORION_ALERT_COOLDOWN_SECONDS", 300),
		AlertRecoveryNotifications:     getEnvBool("ORION_ALERT_RECOVERY_NOTIFICATIONS", true),
		AlertTLSExpiryDays:             getEnvInt("ORION_ALERT_TLS_EXPIRY_DAYS", 14),
		PublicStatusMailEnabled:        getEnvBool("ORION_PUBLIC_STATUS_MAIL_ENABLED", false),
		PublicStatusMailHost:           getEnv("ORION_PUBLIC_STATUS_MAIL_HOST", ""),
		PublicStatusMailPort:           getEnvInt("ORION_PUBLIC_STATUS_MAIL_PORT", 587),
		PublicStatusMailUsername:       getEnv("ORION_PUBLIC_STATUS_MAIL_USERNAME", ""),
		PublicStatusMailPassword:       getEnv("ORION_PUBLIC_STATUS_MAIL_PASSWORD", ""),
		PublicStatusMailFromEmail:      getEnv("ORION_PUBLIC_STATUS_MAIL_FROM_EMAIL", ""),
		PublicStatusMailFromName:       getEnv("ORION_PUBLIC_STATUS_MAIL_FROM_NAME", "Orion Status"),
		PublicStatusMailReplyTo:        getEnv("ORION_PUBLIC_STATUS_MAIL_REPLY_TO", ""),
		PublicStatusURLOrigin:          getEnv("ORION_PUBLIC_STATUS_URL_ORIGIN", ""),
		PublicStatusSubscriberSecret:   getEnv("ORION_PUBLIC_STATUS_SUBSCRIBER_SECRET", ""),
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
	if c.CoreWorkerHeartbeatSeconds <= 0 {
		return &ValidationError{Msg: "ORION_WORKER_HEARTBEAT_SECONDS must be > 0"}
	}
	if c.CoreWorkerStaleSeconds <= 0 {
		return &ValidationError{Msg: "ORION_WORKER_STALE_SECONDS must be > 0"}
	}
	if c.DataLifecycleSchedulerSeconds <= 0 {
		return &ValidationError{Msg: "ORION_DATA_LIFECYCLE_SCHEDULER_SECONDS must be > 0"}
	}
	if c.AlertCooldownSeconds < 0 {
		return &ValidationError{Msg: "ORION_ALERT_COOLDOWN_SECONDS must be >= 0"}
	}
	if c.AlertTLSExpiryDays < 0 {
		return &ValidationError{Msg: "ORION_ALERT_TLS_EXPIRY_DAYS must be >= 0"}
	}
	if c.PublicStatusMailEnabled {
		if strings.TrimSpace(c.PublicStatusMailHost) == "" {
			return &ValidationError{Msg: "ORION_PUBLIC_STATUS_MAIL_HOST is required when ORION_PUBLIC_STATUS_MAIL_ENABLED is true"}
		}
		if c.PublicStatusMailPort <= 0 {
			return &ValidationError{Msg: "ORION_PUBLIC_STATUS_MAIL_PORT must be > 0 when ORION_PUBLIC_STATUS_MAIL_ENABLED is true"}
		}
		if strings.TrimSpace(c.PublicStatusMailFromEmail) == "" {
			return &ValidationError{Msg: "ORION_PUBLIC_STATUS_MAIL_FROM_EMAIL is required when ORION_PUBLIC_STATUS_MAIL_ENABLED is true"}
		}
		if _, err := mail.ParseAddress(strings.TrimSpace(c.PublicStatusMailFromEmail)); err != nil {
			return &ValidationError{Msg: "ORION_PUBLIC_STATUS_MAIL_FROM_EMAIL must be a valid email address"}
		}
		if strings.TrimSpace(c.PublicStatusMailReplyTo) != "" {
			if _, err := mail.ParseAddress(strings.TrimSpace(c.PublicStatusMailReplyTo)); err != nil {
				return &ValidationError{Msg: "ORION_PUBLIC_STATUS_MAIL_REPLY_TO must be a valid email address"}
			}
		}
		if err := validatePublicURLOrigin(c.PublicStatusURLOrigin); err != nil {
			return err
		}
		if strings.TrimSpace(c.PublicStatusSubscriberSecret) == "" {
			return &ValidationError{Msg: "ORION_PUBLIC_STATUS_SUBSCRIBER_SECRET is required when ORION_PUBLIC_STATUS_MAIL_ENABLED is true"}
		}
	}
	return nil
}

// ValidationError is returned when config validation fails.
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }

func validatePublicURLOrigin(value string) error {
	origin := strings.TrimSpace(value)
	if origin == "" {
		return &ValidationError{Msg: "ORION_PUBLIC_STATUS_URL_ORIGIN is required when ORION_PUBLIC_STATUS_MAIL_ENABLED is true"}
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return &ValidationError{Msg: "ORION_PUBLIC_STATUS_URL_ORIGIN must be an absolute URL origin"}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return &ValidationError{Msg: "ORION_PUBLIC_STATUS_URL_ORIGIN must use http or https"}
	}
	if parsed.Path != "" && parsed.Path != "/" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return &ValidationError{Msg: "ORION_PUBLIC_STATUS_URL_ORIGIN must not include path, query, or fragment"}
	}
	return nil
}

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
