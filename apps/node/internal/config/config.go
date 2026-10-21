package config

import (
	"runtime"
	"time"
)

type InternalState struct {
	NodeID            string    `yaml:"node_id"`
	Token             string    `yaml:"token"`
	Registered        bool      `yaml:"registered"`
	CoreURL           string    `yaml:"core_url"`
	LastSync          time.Time `yaml:"last_sync"`
	MaintenanceMode   bool      `yaml:"maintenance_mode"`
	MaintenanceReason *string   `yaml:"maintenance_reason,omitempty"`
	// MaintenanceUntil ends maintenance automatically; nil means the window has no
	// end recorded (state written by an older node).
	MaintenanceUntil *time.Time             `yaml:"maintenance_until,omitempty"`
	Monitors         []InternalStateMonitor `yaml:"monitors"`
}

type InternalStateMonitor struct {
	Name        string    `yaml:"name"`
	ID          string    `yaml:"id"`
	Status      string    `yaml:"status"`
	LastChecked time.Time `yaml:"last_checked"`
}

type UserMonitorType string

const (
	UserMonitorTypeHTTPHealthcheck UserMonitorType = "http-healthcheck"
	UserMonitorInternalService     UserMonitorType = "internal-service"
	UserMonitorTypeCommand         UserMonitorType = "command"
	UserMonitorTypeWebsite         UserMonitorType = "website"
	UserMonitorTypePM2             UserMonitorType = "pm2"
	UserMonitorTypeTCP             UserMonitorType = "tcp"
	UserMonitorTypeResource        UserMonitorType = "resource-threshold"
	UserMonitorTypeDocker          UserMonitorType = "docker-container"
	UserMonitorTypeSystemd         UserMonitorType = "systemd-service"
	UserMonitorTypeDiskUsage       UserMonitorType = "disk-usage"
	UserMonitorTypeProcess         UserMonitorType = "process-running"
)

var UserMonitorTypes = []UserMonitorType{
	UserMonitorTypeHTTPHealthcheck,
	UserMonitorInternalService,
	UserMonitorTypeCommand,
	UserMonitorTypeWebsite,
	UserMonitorTypePM2,
	UserMonitorTypeTCP,
	UserMonitorTypeResource,
	UserMonitorTypeDocker,
	UserMonitorTypeSystemd,
	UserMonitorTypeDiskUsage,
	UserMonitorTypeProcess,
}

type HTTPHealthcheckConfig struct {
	URL               string `yaml:"url"`
	Timeout           string `yaml:"timeout"`
	ExpectedStatus    int    `yaml:"expected_status"`
	ExpectedBody      string `yaml:"expected_body,omitempty"`
	ExpectedBodyRegex string `yaml:"expected_body_regex,omitempty"`
}

type InternalServiceConfig struct {
	Ping    PingConfig    `yaml:"ping"`
	Process ProcessConfig `yaml:"process"`
}

type PingConfig struct {
	URL     string `yaml:"url"`
	Timeout string `yaml:"timeout"`
}

type ProcessConfig struct {
	Port int `yaml:"port"`
}

type CommandMonitorConfig struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args,omitempty"`
	Timeout string   `yaml:"timeout,omitempty"`
}

type WebsiteMonitorConfig struct {
	URL            string `yaml:"url"`
	Timeout        string `yaml:"timeout,omitempty"`
	ExpectedStatus int    `yaml:"expected_status,omitempty"`
}

type PM2MonitorConfig struct {
	AppName string `yaml:"app_name"`
}

type TCPMonitorConfig struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Timeout string `yaml:"timeout,omitempty"`
}

type ResourceThresholdConfig struct {
	MaxCPUPercent    float64 `yaml:"max_cpu_percent,omitempty"`
	MaxMemoryPercent float64 `yaml:"max_memory_percent,omitempty"`
	MaxDiskPercent   float64 `yaml:"max_disk_percent,omitempty"`
	MaxLoad1         float64 `yaml:"max_load_1,omitempty"`
}

type DockerContainerConfig struct {
	Name string `yaml:"name"`
}

// DiskUsageConfig checks the used percent of the filesystem holding Path.
type DiskUsageConfig struct {
	Path       string  `yaml:"path"`
	MaxPercent float64 `yaml:"max_percent"`
}

// ProcessRunningConfig checks that a process with this executable name exists.
type ProcessRunningConfig struct {
	Name string `yaml:"name"`
}

type SystemdServiceConfig struct {
	Name string `yaml:"name"`
}

type UserMonitor struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Type        UserMonitorType        `yaml:"type"`
	Interval    string                 `yaml:"interval"`
	Meta        map[string]interface{} `yaml:"meta,omitempty"`

	HTTP            *HTTPHealthcheckConfig   `yaml:"http,omitempty"`
	InternalService *InternalServiceConfig   `yaml:"internal_service,omitempty"`
	Command         *CommandMonitorConfig    `yaml:"command,omitempty"`
	Website         *WebsiteMonitorConfig    `yaml:"website,omitempty"`
	PM2             *PM2MonitorConfig        `yaml:"pm2,omitempty"`
	TCP             *TCPMonitorConfig        `yaml:"tcp,omitempty"`
	Resource        *ResourceThresholdConfig `yaml:"resource,omitempty"`
	Docker          *DockerContainerConfig   `yaml:"docker,omitempty"`
	Systemd         *SystemdServiceConfig    `yaml:"systemd,omitempty"`
	DiskUsage       *DiskUsageConfig         `yaml:"disk_usage,omitempty"`
	ProcessRunning  *ProcessRunningConfig    `yaml:"process_running,omitempty"`
}

type UserConfig struct {
	Meta    map[string]interface{} `yaml:"meta,omitempty"`
	CoreURL string                 `yaml:"core_url"`
	// EnrollmentToken is the shared secret for first registration when Core sets
	// ORION_ENROLLMENT_TOKEN. The ORION_ENROLLMENT_TOKEN env var overrides it.
	EnrollmentToken string             `yaml:"enrollment_token,omitempty"`
	Interval        string             `yaml:"interval"`
	GeoLocation     bool               `yaml:"geo_location,omitempty"`
	Logging         LoggingConfig      `yaml:"logging,omitempty"`
	Transport       TransportConfig    `yaml:"transport,omitempty"`
	AppResources    AppResourcesConfig `yaml:"app_resources,omitempty"`
	Logs            LogsConfig         `yaml:"logs,omitempty"`
	Discovery       DiscoveryConfig    `yaml:"discovery,omitempty"`
	Monitors        []UserMonitor      `yaml:"monitors"`
}

// DiscoveryConfig controls the read-only scan that proposes what to monitor.
// Hand-written config is never changed by it, and what you decline stays
// declined.
type DiscoveryConfig struct {
	Enabled *bool    `yaml:"enabled,omitempty"` // default true
	Kinds   []string `yaml:"kinds,omitempty"`   // default: all of services, containers, pm2, ports, disks, logs, storage
	Ignore  []string `yaml:"ignore,omitempty"`  // suggestion ids never to propose again, e.g. port:5432
	// Report sends the latest scan (names of services, containers, ports, disks
	// and log files found here, never their contents) to Core so the Console can
	// show "Detected on this server". Default true; false keeps the scan local.
	Report *bool `yaml:"report,omitempty"`
}

// IsReporting says whether scan results are sent to Core (default yes).
func (d DiscoveryConfig) IsReporting() bool { return d.Report == nil || *d.Report }

// IsEnabled reports whether scanning is on (default on).
func (d DiscoveryConfig) IsEnabled() bool { return d.Enabled == nil || *d.Enabled }

// KindEnabled reports whether a kind is scanned.
func (d DiscoveryConfig) KindEnabled(kind string) bool {
	if len(d.Kinds) == 0 {
		return true
	}
	for _, k := range d.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// LogsConfig ships logs from this host to Core's product-log ingest
// (docs/product-logs.md). Sources are read from files, journald or containers;
// everything goes through a bounded on-disk spool first.
type LogsConfig struct {
	SpoolMB  int         `yaml:"spool_mb,omitempty"` // default 64
	Endpoint string      `yaml:"endpoint,omitempty"` // default {core_url}/v1/ingest/logs
	Sources  []LogSource `yaml:"sources,omitempty"`
}

// LogSource is one thing to read logs from. Exactly one of Path, Unit or
// Container applies, chosen by Type.
type LogSource struct {
	Name           string   `yaml:"name"`
	Type           string   `yaml:"type"` // file | journald | docker
	Path           string   `yaml:"path,omitempty"`
	Unit           string   `yaml:"unit,omitempty"`
	Container      string   `yaml:"container,omitempty"`
	App            string   `yaml:"app,omitempty"`  // service.name; defaults to Name
	From           string   `yaml:"from,omitempty"` // "end" (default) or "beginning"
	MultilineStart string   `yaml:"multiline_start,omitempty"`
	Redact         []string `yaml:"redact,omitempty"`
	MinLevel       string   `yaml:"min_level,omitempty"`
	// IngestKeyFile or IngestKeyEnv names where this source's write-only ingest
	// key lives; the key itself never goes in the config file.
	IngestKeyFile string `yaml:"ingest_key_file,omitempty"`
	IngestKeyEnv  string `yaml:"ingest_key_env,omitempty"`
}

// AppResourcesConfig controls passive per-app resource metrics. They need no
// app integration and are on by default; caps keep cardinality bounded.
type AppResourcesConfig struct {
	Enabled *bool    `yaml:"enabled,omitempty"` // default true
	TopN    int      `yaml:"top_n,omitempty"`   // processes per ranking, default 10
	Paths   []string `yaml:"paths,omitempty"`   // storage paths to size
	// Docker adds container names and per-container network from the Docker API
	// when the node user can open the socket. Default on; false never touches it.
	Docker       *bool  `yaml:"docker,omitempty"`
	DockerSocket string `yaml:"docker_socket,omitempty"`
}

// DockerEnabled reports whether the Docker API may be used (default yes, and
// it only yields data where the socket is readable).
func (a AppResourcesConfig) DockerEnabled() bool { return a.Docker == nil || *a.Docker }

// IsEnabled reports whether passive app metrics are on (default on).
func (a AppResourcesConfig) IsEnabled() bool { return a.Enabled == nil || *a.Enabled }

// TransportConfig tunes how the node talks to Core.
type TransportConfig struct {
	// Compression is "" or "none" (default) or "gzip". gzip sends request
	// bodies over 1 KiB with Content-Encoding: gzip, which Core must accept.
	Compression string `yaml:"compression,omitempty"`
}

type LoggingConfig struct {
	Level      string `yaml:"level,omitempty"`
	Path       string `yaml:"path,omitempty"`
	Format     string `yaml:"format,omitempty"`
	MaxSizeMB  int    `yaml:"max_size_mb,omitempty"`
	MaxBackups int    `yaml:"max_backups,omitempty"`
	MaxAgeDays int    `yaml:"max_age_days,omitempty"`
	Compress   *bool  `yaml:"compress,omitempty"`
}

// LoadUserConfig loads the effective config (file, config.d drop-ins, env).
func LoadUserConfig(path string) (*UserConfig, error) {
	cfg, _, err := Load(path)
	return cfg, err
}

func (c *UserConfig) ApplyDefaults() {
	c.Logging = c.Logging.WithDefaults()
}

func DefaultLoggingConfig() LoggingConfig {
	compress := true
	return LoggingConfig{
		Level:      "info",
		Path:       DefaultLogPath(),
		Format:     "json",
		MaxSizeMB:  25,
		MaxBackups: 5,
		MaxAgeDays: 14,
		Compress:   &compress,
	}
}

func (l LoggingConfig) WithDefaults() LoggingConfig {
	defaults := DefaultLoggingConfig()
	if l.Level == "" {
		l.Level = defaults.Level
	}
	if l.Path == "" {
		l.Path = defaults.Path
	}
	if l.Format == "" {
		l.Format = defaults.Format
	}
	if l.MaxSizeMB == 0 {
		l.MaxSizeMB = defaults.MaxSizeMB
	}
	if l.MaxBackups == 0 {
		l.MaxBackups = defaults.MaxBackups
	}
	if l.MaxAgeDays == 0 {
		l.MaxAgeDays = defaults.MaxAgeDays
	}
	if l.Compress == nil {
		l.Compress = defaults.Compress
	}
	return l
}

func (l LoggingConfig) CompressEnabled() bool {
	if l.Compress == nil {
		return true
	}
	return *l.Compress
}

func (c *InternalState) IsRegistered() bool {
	return c.NodeID != "" && c.Token != ""
}

// DefaultPath returns the default path for the node config.
// On Linux: /etc/orion/config.yaml
// On macOS: /usr/local/etc/orion/config.yaml
func DefaultPath() string {
	switch runtime.GOOS {
	case "linux":
		return "/etc/orion/config.yaml"
	case "darwin":
		return "/usr/local/etc/orion/config.yaml"
	default:
		return "config.yaml"
	}
}

// DefaultLogPath returns the default structured runtime log path.
// On Linux: /var/log/orion/node.log
// On macOS: /usr/local/var/log/orion/node.log
func DefaultLogPath() string {
	switch runtime.GOOS {
	case "linux":
		return "/var/log/orion/node.log"
	case "darwin":
		return "/usr/local/var/log/orion/node.log"
	default:
		return "node.log"
	}
}
