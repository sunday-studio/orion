package config

import (
	"errors"
	"fmt"
	"time"
)

func (h HTTPHealthcheckConfig) Validate() error {
	if h.URL == "" {
		return errors.New("url is required")
	}

	if h.Timeout != "" {
		if _, err := time.ParseDuration(h.Timeout); err != nil {
			return fmt.Errorf("invalid timeout: %w", err)
		}
	}

	return nil
}

func (i InternalServiceConfig) Validate() error {
	if i.Ping.URL == "" {
		return errors.New("ping.url is required")
	}

	if i.Ping.Timeout == "" {
		return errors.New("ping.timeout is required")
	}

	if _, err := time.ParseDuration(i.Ping.Timeout); err != nil {
		return fmt.Errorf("invalid ping.timeout: %w", err)
	}

	if i.Process.Port <= 0 {
		return errors.New("process.port must be > 0")
	}

	return nil
}

func (c CommandMonitorConfig) Validate() error {
	if c.Command == "" {
		return errors.New("cmd is required")
	}
	return nil
}

func (w WebsiteMonitorConfig) Validate() error {
	if w.URL == "" {
		return errors.New("url is required")
	}

	if w.Timeout != "" {
		if _, err := time.ParseDuration(w.Timeout); err != nil {
			return fmt.Errorf("invalid timeout: %w", err)
		}
	}

	if w.ExpectedStatus < 0 {
		return errors.New("expected_status must be >= 0")
	}

	return nil
}

func (p PM2MonitorConfig) Validate() error {
	if p.AppName == "" {
		return errors.New("app_name is required")
	}
	return nil
}

func (m UserMonitor) Validate() error {
	if m.Name == "" {
		return errors.New("name is required")
	}

	if m.Type == "" {
		return errors.New("type is required")
	}

	if m.Interval == "" {
		return errors.New("interval is required")
	}

	if _, err := time.ParseDuration(m.Interval); err != nil {
		return fmt.Errorf("invalid interval: %w", err)
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
	if c.CoreURL == "" {
		return errors.New("core_url is required")
	}

	if c.Interval == "" {
		return errors.New("interval is required")
	}

	if _, err := time.ParseDuration(c.Interval); err != nil {
		return fmt.Errorf("invalid interval format: %w", err)
	}

	for i, monitor := range c.Monitors {
		if err := monitor.Validate(); err != nil {
			return fmt.Errorf("monitor[%d] (%s): %w", i, monitor.Name, err)
		}
	}

	return nil
}
