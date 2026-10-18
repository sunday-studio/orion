package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"orion/agent/internal/config"

	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// wellKnown services are almost always worth monitoring.
var wellKnown = map[string]bool{
	"nginx": true, "apache2": true, "httpd": true, "caddy": true, "traefik": true, "haproxy": true,
	"postgresql": true, "postgres": true, "mysql": true, "mysqld": true, "mariadb": true, "mongod": true, "redis": true, "redis-server": true,
	"rabbitmq-server": true, "kafka": true, "elasticsearch": true, "docker": true, "containerd": true, "php-fpm": true, "memcached": true,
}

// noise is infrastructure nobody monitors on purpose.
var noisePrefixes = []string{"systemd-", "dbus", "getty", "user@", "polkit", "udisks", "ModemManager", "NetworkManager", "multipathd",
	"snapd", "unattended-upgrades", "cron", "rsyslog", "accounts-daemon", "avahi", "bluetooth", "cups", "irqbalance", "lvm2", "packagekit", "serial-getty"}

func isNoise(name string) bool {
	for _, p := range noisePrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func newMonitor(e Env, name string, t config.UserMonitorType, fill func(*config.UserMonitor)) *config.UserMonitor {
	m := &config.UserMonitor{Name: name, Type: t, Interval: e.interval()}
	fill(m)
	return m
}

func systemdUnits(ctx context.Context, e Env) ([]Candidate, error) {
	out, err := e.Run(ctx, "systemctl", "list-units", "--type=service", "--state=running", "--no-legend", "--plain", "--no-pager")
	if err != nil {
		return nil, fmt.Errorf("systemctl: %w", err)
	}
	var cs []Candidate
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 || !strings.HasSuffix(f[0], ".service") {
			continue
		}
		unit := strings.TrimSuffix(f[0], ".service")
		if isNoise(unit) {
			continue
		}
		conf := Medium
		if wellKnown[unit] || wellKnown[strings.SplitN(unit, "@", 2)[0]] {
			conf = High
		}
		name := "svc-" + unit
		cs = append(cs, Candidate{
			ID: "service:" + unit, Kind: KindService, Name: unit, Confidence: conf, Detail: "systemd unit, running",
			Monitor: newMonitor(e, name, config.UserMonitorTypeSystemd, func(m *config.UserMonitor) {
				m.Description = "systemd unit " + unit
				m.Systemd = &config.SystemdServiceConfig{Name: unit}
			}),
		})
	}
	return cs, nil
}

func launchdServices(ctx context.Context, e Env) ([]Candidate, error) {
	out, err := e.Run(ctx, "launchctl", "list")
	if err != nil {
		return nil, fmt.Errorf("launchctl: %w", err)
	}
	var cs []Candidate
	for i, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if i == 0 || len(f) < 3 || f[0] == "-" { // header, or not running
			continue
		}
		label := f[2]
		if strings.HasPrefix(label, "com.apple.") || strings.HasPrefix(label, "application.") || strings.HasPrefix(label, "0x") {
			continue
		}
		cs = append(cs, Candidate{
			ID: "service:" + label, Kind: KindService, Name: label, Confidence: Medium, Detail: "launchd job, pid " + f[0],
			// No launchd monitor type exists yet: surfaced for resource metrics, not proposed as a check.
		})
	}
	return cs, nil
}

func dockerContainers(ctx context.Context, e Env) ([]Candidate, error) {
	out, err := e.Run(ctx, "docker", "ps", "--format", "{{json .}}")
	if err != nil {
		return nil, fmt.Errorf("docker ps (is the service user in the docker group?): %w", err)
	}
	var cs []Candidate
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var row struct{ Names, Image, Status string }
		if json.Unmarshal([]byte(line), &row) != nil || row.Names == "" {
			continue
		}
		name := strings.Split(row.Names, ",")[0]
		cs = append(cs, Candidate{
			ID: "container:" + name, Kind: KindContainer, Name: name, Confidence: High, Detail: row.Image + ", " + row.Status,
			Monitor: newMonitor(e, "ctr-"+name, config.UserMonitorTypeDocker, func(m *config.UserMonitor) {
				m.Description = "docker container " + name
				m.Docker = &config.DockerContainerConfig{Name: name}
			}),
		})
	}
	return cs, nil
}

func pm2Apps(ctx context.Context, e Env) ([]Candidate, error) {
	out, err := e.Run(ctx, "pm2", "jlist")
	if err != nil {
		return nil, fmt.Errorf("pm2 jlist: %w", err)
	}
	var apps []struct {
		Name string `json:"name"`
		Pm2  struct {
			Status string `json:"status"`
		} `json:"pm2_env"`
	}
	// pm2 prints banners before the JSON array on some versions.
	text := string(out)
	if i := strings.Index(text, "[{"); i >= 0 {
		out = []byte(text[i:])
	} else if i := strings.Index(text, "[]"); i >= 0 {
		out = []byte(text[i:])
	}
	if err := json.Unmarshal(out, &apps); err != nil {
		return nil, fmt.Errorf("parse pm2 output: %w", err)
	}
	var cs []Candidate
	for _, a := range apps {
		cs = append(cs, Candidate{
			ID: "pm2:" + a.Name, Kind: KindPM2, Name: a.Name, Confidence: High, Detail: "pm2 app, " + a.Pm2.Status,
			Monitor: newMonitor(e, "pm2-"+a.Name, config.UserMonitorTypePM2, func(m *config.UserMonitor) {
				m.Description = "pm2 app " + a.Name
				m.PM2 = &config.PM2MonitorConfig{AppName: a.Name}
			}),
		})
	}
	return cs, nil
}

func hostPorts(ctx context.Context) ([]ListeningPort, error) {
	conns, err := net.ConnectionsWithContext(ctx, "tcp")
	if err != nil {
		return nil, err
	}
	var out []ListeningPort
	for _, c := range conns {
		if c.Status != "LISTEN" || c.Laddr.Port == 0 {
			continue
		}
		name := ""
		if c.Pid > 0 {
			if p, err := process.NewProcessWithContext(ctx, c.Pid); err == nil {
				name, _ = p.NameWithContext(ctx)
			}
		}
		out = append(out, ListeningPort{Port: int(c.Laddr.Port), Process: name})
	}
	return out, nil
}

func listeningPorts(ctx context.Context, e Env) ([]Candidate, error) {
	if e.Ports == nil {
		return nil, nil
	}
	ports, err := e.Ports(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing sockets: %w", err)
	}
	var cs []Candidate
	for _, p := range ports {
		if p.Port == 22 || p.Port == 53 { // ssh and local resolvers are not "apps"
			continue
		}
		conf := Medium
		if wellKnown[strings.ToLower(p.Process)] {
			conf = High
		}
		proc := p.Process
		if proc == "" {
			proc, conf = "unknown process", Low
		}
		port := p.Port
		cs = append(cs, Candidate{
			ID: "port:" + strconv.Itoa(port), Kind: KindPort, Name: fmt.Sprintf(":%d", port), Confidence: conf, Detail: "listening, " + proc,
			Monitor: newMonitor(e, fmt.Sprintf("port-%d", port), config.UserMonitorTypeTCP, func(m *config.UserMonitor) {
				m.Description = fmt.Sprintf("TCP port %d (%s)", port, proc)
				m.TCP = &config.TCPMonitorConfig{Host: "127.0.0.1", Port: port, Timeout: "3s"}
			}),
		})
	}
	return cs, nil
}

var logGlobs = []string{
	"/var/log/nginx/error.log", "/var/log/nginx/access.log", "/var/log/apache2/error.log", "/var/log/httpd/error_log",
	"/var/log/postgresql/*.log", "/var/log/mysql/error.log", "/var/log/redis/*.log", "/var/log/caddy/*.log",
	"/var/log/syslog", "/var/log/messages", "/var/log/auth.log",
	"/usr/local/var/log/*.log",
}

func logFiles(e Env) []Candidate {
	if e.Glob == nil {
		return nil
	}
	var cs []Candidate
	for _, g := range logGlobs {
		matches, _ := e.Glob(g)
		for _, m := range matches {
			conf := Medium
			if base := filepath.Base(m); base == "syslog" || base == "messages" || base == "auth.log" {
				conf = Low
			}
			cs = append(cs, Candidate{ID: "log:" + m, Kind: KindLog, Name: m, Confidence: conf, Detail: "log file", LogPath: m})
		}
	}
	return cs
}
