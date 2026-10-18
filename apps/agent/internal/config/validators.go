package config

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
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
	if len(c.Args) > 0 && strings.ContainsAny(c.Command, " \t\r\n") {
		return errors.New("command must be an executable path/name without spaces when args are provided")
	}

	if c.Timeout != "" {
		if _, err := parsePositiveDuration(c.Timeout, "timeout"); err != nil {
			return err
		}
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

func (t TCPMonitorConfig) Validate() error {
	if strings.TrimSpace(t.Host) == "" {
		return errors.New("host is required")
	}

	if t.Port <= 0 || t.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}

	if t.Timeout != "" {
		if _, err := parsePositiveDuration(t.Timeout, "timeout"); err != nil {
			return err
		}
	}

	return nil
}

func (r ResourceThresholdConfig) Validate() error {
	if r.MaxCPUPercent == 0 && r.MaxMemoryPercent == 0 && r.MaxDiskPercent == 0 && r.MaxLoad1 == 0 {
		return errors.New("at least one resource threshold is required")
	}

	if err := validatePercentThreshold(r.MaxCPUPercent, "max_cpu_percent"); err != nil {
		return err
	}
	if err := validatePercentThreshold(r.MaxMemoryPercent, "max_memory_percent"); err != nil {
		return err
	}
	if err := validatePercentThreshold(r.MaxDiskPercent, "max_disk_percent"); err != nil {
		return err
	}
	if r.MaxLoad1 < 0 {
		return errors.New("max_load_1 must be >= 0")
	}

	return nil
}

func (d DockerContainerConfig) Validate() error {
	if strings.TrimSpace(d.Name) == "" {
		return errors.New("name is required")
	}
	return nil
}

func (s SystemdServiceConfig) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return errors.New("name is required")
	}
	return nil
}

func (l LoggingConfig) Validate() error {
	normalized := l.WithDefaults()

	switch strings.ToLower(normalized.Level) {
	case "debug", "info", "warn", "error":
	default:
		return errors.New("logging.level must be one of debug, info, warn, error")
	}

	if strings.TrimSpace(normalized.Path) == "" {
		return errors.New("logging.path is required")
	}
	if (runtime.GOOS == "linux" || runtime.GOOS == "darwin") && !IsConsoleLogPath(normalized.Path) && !filepath.IsAbs(normalized.Path) {
		return errors.New("logging.path must be absolute")
	}

	switch strings.ToLower(normalized.Format) {
	case "json":
	default:
		return errors.New("logging.format must be json")
	}

	if normalized.MaxSizeMB < 0 {
		return errors.New("logging.max_size_mb must be >= 0")
	}
	if normalized.MaxBackups < 0 {
		return errors.New("logging.max_backups must be >= 0")
	}
	if normalized.MaxAgeDays < 0 {
		return errors.New("logging.max_age_days must be >= 0")
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

	case UserMonitorTypeTCP:
		if m.TCP == nil {
			return errors.New("tcp config is required")
		}
		return m.TCP.Validate()

	case UserMonitorTypeResource:
		if m.Resource == nil {
			return errors.New("resource config is required")
		}
		return m.Resource.Validate()

	case UserMonitorTypeDocker:
		if m.Docker == nil {
			return errors.New("docker config is required")
		}
		return m.Docker.Validate()

	case UserMonitorTypeSystemd:
		if m.Systemd == nil {
			return errors.New("systemd config is required")
		}
		return m.Systemd.Validate()

	case UserMonitorTypeDiskUsage:
		if m.DiskUsage == nil {
			return errors.New("disk_usage config is required")
		}
		return m.DiskUsage.Validate()

	case UserMonitorTypeProcess:
		if m.ProcessRunning == nil {
			return errors.New("process_running config is required")
		}
		return m.ProcessRunning.Validate()

	default:
		return fmt.Errorf("unsupported monitor type: %s", m.Type)
	}
}

func (d DiskUsageConfig) Validate() error {
	if !filepath.IsAbs(d.Path) {
		return errors.New("path must be an absolute path")
	}
	return validatePercentThreshold(d.MaxPercent, "max_percent")
}

func (p ProcessRunningConfig) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("name is required")
	}
	return nil
}

func (c *UserConfig) Validate() error {
	if err := validateHTTPURL(c.CoreURL, "core_url"); err != nil {
		return err
	}

	if err := c.Logging.Validate(); err != nil {
		return err
	}
	if c.AppResources.TopN < 0 || c.AppResources.TopN > 100 {
		return errors.New("app_resources.top_n must be between 0 and 100")
	}
	for _, p := range c.AppResources.Paths {
		if !filepath.IsAbs(p) {
			return fmt.Errorf("app_resources.paths entry %q must be an absolute path", p)
		}
	}
	if err := c.Logs.Validate(); err != nil {
		return err
	}
	switch strings.ToLower(c.Transport.Compression) {
	case "", "none", "gzip":
	default:
		return errors.New("transport.compression must be none or gzip")
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

func validatePercentThreshold(value float64, field string) error {
	if value < 0 || value > 100 {
		return fmt.Errorf("%s must be between 0 and 100", field)
	}
	return nil
}

// IsConsoleLogPath reports whether a logging.path value means "console only".
func IsConsoleLogPath(p string) bool {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "stdout", "stderr", "-", "none":
		return true
	}
	return false
}

// Validate checks the log sources.
func (l LogsConfig) Validate() error {
	if l.SpoolMB < 0 || l.SpoolMB > 4096 {
		return errors.New("logs.spool_mb must be between 0 and 4096")
	}
	seen := map[string]bool{}
	for i, s := range l.Sources {
		at := fmt.Sprintf("logs.sources[%d]", i)
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("%s: name is required", at)
		}
		if seen[s.Name] {
			return fmt.Errorf("%s: duplicate source name %q", at, s.Name)
		}
		seen[s.Name] = true
		switch s.Type {
		case "file":
			if !filepath.IsAbs(s.Path) {
				return fmt.Errorf("%s (%s): path must be an absolute path or glob", at, s.Name)
			}
		case "journald":
			if strings.TrimSpace(s.Unit) == "" {
				return fmt.Errorf("%s (%s): unit is required for journald", at, s.Name)
			}
		case "docker":
			if strings.TrimSpace(s.Container) == "" {
				return fmt.Errorf("%s (%s): container is required for docker", at, s.Name)
			}
		default:
			return fmt.Errorf("%s (%s): type must be file, journald or docker", at, s.Name)
		}
		if s.From != "" && s.From != "end" && s.From != "beginning" {
			return fmt.Errorf("%s (%s): from must be end or beginning", at, s.Name)
		}
		if s.MultilineStart != "" {
			if _, err := regexp.Compile(s.MultilineStart); err != nil {
				return fmt.Errorf("%s (%s): invalid multiline_start: %w", at, s.Name, err)
			}
		}
		if s.IngestKeyFile == "" && s.IngestKeyEnv == "" {
			return fmt.Errorf("%s (%s): set ingest_key_file or ingest_key_env (Console, Settings, Apps, create a key)", at, s.Name)
		}
	}
	return nil
}
