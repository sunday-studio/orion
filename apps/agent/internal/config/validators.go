package config

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func (h HTTPHealthcheckConfig) Validate() error {
	if err := validateHTTPURL(h.URL, "url"); err != nil {
		return err
	}

	if h.Timeout == "" {
		return errors.New("timeout is required")
	}
	if _, err := parsePositiveDuration(h.Timeout, "timeout"); err != nil {
		return err
	}

	if err := validateHTTPStatus(h.ExpectedStatus, true); err != nil {
		return err
	}

	if h.ExpectedBodyRegex != "" {
		if _, err := regexp.Compile(h.ExpectedBodyRegex); err != nil {
			return fmt.Errorf("invalid expected_body_regex: %w", err)
		}
	}

	return nil
}

func (i InternalServiceConfig) Validate() error {
	if err := validateHTTPURL(i.Ping.URL, "ping.url"); err != nil {
		return err
	}

	if i.Ping.Timeout == "" {
		return errors.New("ping.timeout is required")
	}
	if _, err := parsePositiveDuration(i.Ping.Timeout, "ping.timeout"); err != nil {
		return err
	}

	if i.Process.Port <= 0 || i.Process.Port > 65535 {
		return errors.New("process.port must be between 1 and 65535")
	}

	return nil
}

func (c CommandMonitorConfig) Validate() error {
	if strings.TrimSpace(c.Command) == "" {
		return errors.New("command is required")
	}
	return nil
}

func (w WebsiteMonitorConfig) Validate() error {
	if err := validateHTTPURL(w.URL, "url"); err != nil {
		return err
	}

	if w.Timeout != "" {
		if _, err := parsePositiveDuration(w.Timeout, "timeout"); err != nil {
			return err
		}
	}

	return validateHTTPStatus(w.ExpectedStatus, false)
}

func (p PM2MonitorConfig) Validate() error {
	if strings.TrimSpace(p.AppName) == "" {
		return errors.New("app_name is required")
	}
	return nil
}

func (m UserMonitor) Validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return errors.New("name is required")
	}

	if m.Type == "" {
		return errors.New("type is required")
	}

	if m.Interval == "" {
		return errors.New("interval is required")
	}
	if _, err := parsePositiveDuration(m.Interval, "interval"); err != nil {
		return err
	}

	switch m.Type {
	case UserMonitorTypeHTTPHealthcheck:
		if m.HTTP == nil {
			return errors.New("http config is required for http-healthcheck")
		}
		return m.HTTP.Validate()

	case UserMonitorInternalService:
		if m.InternalService == nil {
			return errors.New("internal_service config is required")
		}
		return m.InternalService.Validate()

	case UserMonitorTypeCommand:
		if m.Command == nil {
			return errors.New("command config is required")
		}
		return m.Command.Validate()

	case UserMonitorTypeWebsite:
		if m.Website == nil {
			return errors.New("website config is required")
		}
		return m.Website.Validate()

	case UserMonitorTypePM2:
		if m.PM2 == nil {
			return errors.New("pm2 config is required")
		}
		return m.PM2.Validate()

	default:
		return fmt.Errorf("unsupported monitor type: %s", m.Type)
	}
}

func (c *UserConfig) Validate() error {
	if err := validateHTTPURL(c.CoreURL, "core_url"); err != nil {
		return err
	}

	if c.Interval == "" {
		return errors.New("interval is required")
	}
	if _, err := parsePositiveDuration(c.Interval, "interval"); err != nil {
		return err
	}

	names := make(map[string]int, len(c.Monitors))
	for i, monitor := range c.Monitors {
		if err := monitor.Validate(); err != nil {
			return fmt.Errorf("monitor[%d] (%s): %w", i, monitor.Name, err)
		}

		normalizedName := strings.TrimSpace(monitor.Name)
		if firstIndex, exists := names[normalizedName]; exists {
			return fmt.Errorf("monitor[%d] (%s): duplicate name also used by monitor[%d]", i, monitor.Name, firstIndex)
		}
		names[normalizedName] = i
	}

	return nil
}

func validateHTTPURL(rawURL string, field string) error {
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("%s is required", field)
	}

	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute http or https URL", field)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", field)
	}

	return nil
}

func parsePositiveDuration(rawDuration string, field string) (time.Duration, error) {
	duration, err := time.ParseDuration(rawDuration)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", field, err)
	}

	if duration <= 0 {
		return 0, fmt.Errorf("%s must be > 0", field)
	}

	return duration, nil
}

func validateHTTPStatus(status int, required bool) error {
	if status == 0 && !required {
		return nil
	}

	if status == 0 && required {
		return errors.New("expected_status is required")
	}

	if status < 100 || status > 599 {
		return errors.New("expected_status must be between 100 and 599")
	}

	return nil
}
