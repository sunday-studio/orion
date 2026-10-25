package config

import (
	"os"
	"strconv"
)

// Config holds runtime configuration from environment.
type Config struct {
	DataDir        string
	Port           string
	AdminUsername  string
	AdminPassword  string
	JWTSecret      string
	FrontendAuthOn bool

	// RetentionDays is how long raw reports stay in the main DB (0 disables pruning).
	RetentionDays int
	// ArchiveEnabled copies pruned reports into archive.db instead of discarding them.
	ArchiveEnabled bool
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
		DataDir:        getEnv("ORION_DATA_DIR", "data"),
		Port:           getEnv("ORION_PORT", "8999"),
		AdminUsername:  adminUser,
		AdminPassword:  adminPass,
		JWTSecret:      jwtSecret,
		FrontendAuthOn: frontendAuthOn,
		RetentionDays:  getEnvInt("ORION_RETENTION_DAYS", 30),
		ArchiveEnabled: getEnv("ORION_ARCHIVE_ENABLED", "true") != "false",
	}
}

// Validate returns an error if config is invalid (e.g. frontend auth on but JWT_SECRET empty).
func (c *Config) Validate() error {
	if c.FrontendAuthOn && c.JWTSecret == "" {
		return &ValidationError{Msg: "ORION_JWT_SECRET is required when ORION_ADMIN_USERNAME and ORION_ADMIN_PASSWORD are set"}
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

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return fallback
}
