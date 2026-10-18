package doctor

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// HTTPDoer is the subset of http.Client the checks use.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// ClockLimit is the largest tolerated difference between this host and Core.
const ClockLimit = 5 * time.Second

// DefaultChecks is the standard check list, in the order the artboard shows.
func DefaultChecks() []Check {
	return []Check{
		{"config", "Config", checkConfig},
		{"dns", "DNS", checkDNS},
		{"tls", "TLS", checkTLS},
		{"core", "Core reachable", checkCore},
		{"token", "Token", checkToken},
		{"clock", "Clock skew", checkClock},
		{"log-access", "Log access", checkLogAccess},
		{"log-sources", "Log sources", checkLogSources},
		{"disk", "Disk space", checkDisk},
		{"service", "Service", checkService},
		{"version", "Version", checkVersion},
		{"spool", "Spool", checkSpool},
	}
}

func coreURL(e *Env) (*url.URL, bool) {
	if e.Config == nil {
		return nil, false
	}
	u, err := url.Parse(e.Config.CoreURL)
	if err != nil || u.Host == "" {
		return nil, false
	}
	return u, true
}

func firstLine(s string) string { return strings.SplitN(s, "\n", 2)[0] }

func checkConfig(_ context.Context, e *Env) Result {
	if e.ConfigErr != nil {
		return fail(firstLine(e.ConfigErr.Error()), "orion-agent setup   (or fix the file: orion-agent config edit)")
	}
	n := len(e.Config.Monitors)
	return ok(fmt.Sprintf("%s, %d %s", e.ConfigPath, n, plural(n, "monitor", "monitors")))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func checkService(_ context.Context, e *Env) Result {
	if e.Service == nil {
		return skip("no service manager detected")
	}
	installed, running, state := e.Service()
	switch {
	case !installed:
		return fail("not installed", "orion-agent service install   (prints the one-line installer)")
	case !running:
		start := "sudo systemctl start orion-agent"
		if e.GOOS == "darwin" {
			start = "sudo launchctl bootstrap system /Library/LaunchDaemons/com.orion.agent.plist"
		}
		return fail(state, start+"   (why it stopped: orion-agent logs)")
	}
	if !e.Daemon.Reachable {
		msg := "running, control socket not reachable"
		if e.Daemon.Err != nil && strings.Contains(e.Daemon.Err.Error(), "permission") {
			return warn(msg, "sudo usermod -aG orion $USER   (then log out and in)")
		}
		return warn(msg, "orion-agent restart")
	}
	if e.Daemon.Uptime > 0 {
		return ok("active, running " + Duration(e.Daemon.Uptime))
	}
	return ok("active, running")
}

// Duration formats like the artboards: "41 s", "5 m", "3d 4h".
func Duration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d m", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
}

func checkVersion(_ context.Context, e *Env) Result {
	if e.Daemon.Reachable && e.Daemon.Version != e.Version {
		return warn(fmt.Sprintf("cli %s, daemon %s", e.Version, e.Daemon.Version), "orion-agent restart")
	}
	return ok(e.Version)
}

func checkDNS(ctx context.Context, e *Env) Result {
	u, good := coreURL(e)
	if !good {
		return skip("no core_url yet")
	}
	host := u.Hostname()
	if net.ParseIP(host) != nil {
		return ok(host + " is an IP address")
	}
	lookup := e.LookupHost
	if lookup == nil {
		lookup = net.DefaultResolver.LookupHost
	}
	cctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	addrs, err := lookup(cctx, host)
	if err != nil || len(addrs) == 0 {
		return fail("cannot resolve "+host, "getent hosts "+host+"   (check /etc/resolv.conf, or use the Core IP in core_url)")
	}
	return ok(fmt.Sprintf("%s resolves to %s", host, addrs[0]))
}

func checkTLS(ctx context.Context, e *Env) Result {
	u, good := coreURL(e)
	if !good || u.Scheme != "https" {
		return skip("core_url is not https")
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	dial := e.DialTLS
	if dial == nil {
		dial = defaultDialTLS
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	notAfter, err := dial(cctx, net.JoinHostPort(u.Hostname(), port), u.Hostname())
	if err != nil {
		return fail(firstLine(err.Error()), "private CA? sudo cp ca.crt /usr/local/share/ca-certificates/ && sudo update-ca-certificates   (macOS: sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ca.crt)")
	}
	left := notAfter.Sub(e.now())
	switch {
	case left < 0:
		return fail("expired "+notAfter.Format("Jan 2, 2006"), "renew the certificate on the Core host")
	case left < 14*24*time.Hour:
		return warn("valid until "+notAfter.Format("Jan 2, 2006"), "renew the certificate on the Core host")
	}
	return ok("valid until " + notAfter.Format("Jan 2, 2006"))
}

func defaultDialTLS(ctx context.Context, addr, serverName string) (time.Time, error) {
	d := tls.Dialer{Config: &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12}}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return time.Time{}, err
	}
	defer conn.Close()
	certs := conn.(*tls.Conn).ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return time.Time{}, fmt.Errorf("no certificate presented")
	}
	return certs[0].NotAfter, nil
}

func (e *Env) get(ctx context.Context, path, bearer string) (*http.Response, time.Duration, error) {
	base := strings.TrimRight(e.Config.CoreURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return nil, 0, err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	client := e.HTTP
	if client == nil {
		client = &http.Client{Timeout: 6 * time.Second}
	}
	start := time.Now()
	resp, err := client.Do(req)
	return resp, time.Since(start), err
}

func checkCore(ctx context.Context, e *Env) Result {
	if _, good := coreURL(e); !good {
		return skip("no core_url yet")
	}
	resp, took, err := e.get(ctx, "/health", "")
	if err != nil {
		return fail("cannot connect", "curl -v "+strings.TrimRight(e.Config.CoreURL, "/")+"/health   (firewall, proxy, or wrong core_url?)")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fail(fmt.Sprintf("Core answered HTTP %d", resp.StatusCode), "check Core's health and logs on the Core host")
	}
	return ok(fmt.Sprintf("%d ms", took.Milliseconds()))
}

func checkClock(ctx context.Context, e *Env) Result {
	if _, good := coreURL(e); !good {
		return skip("no core_url yet")
	}
	resp, _, err := e.get(ctx, "/health", "")
	if err != nil {
		return skip("Core not reachable")
	}
	defer resp.Body.Close()
	date, perr := http.ParseTime(resp.Header.Get("Date"))
	if perr != nil {
		return skip("Core sent no Date header")
	}
	skew := e.now().Sub(date) // positive: this host is ahead
	abs := skew
	if abs < 0 {
		abs = -abs
	}
	msg := fmt.Sprintf("%+.1f s (limit %d s)", skew.Seconds(), int(ClockLimit.Seconds()))
	fix := "sudo timedatectl set-ntp true   (macOS: sudo sntp -sS time.apple.com)"
	switch {
	case abs > ClockLimit:
		return fail(msg, fix)
	case abs > ClockLimit/2:
		return warn(msg, fix)
	}
	return ok(msg)
}

func checkToken(ctx context.Context, e *Env) Result {
	if !e.Registered {
		return warn("host not enrolled", "orion-agent enroll --core-url <url> --token-file <file>   (command from Console, Servers, Add server)")
	}
	if e.TokenProbe == nil {
		return skip("cannot check from here; run as a user in the orion group")
	}
	code, err := e.TokenProbe(ctx)
	if err != nil {
		return skip("Core not reachable")
	}
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return fail(fmt.Sprintf("rejected by Core (HTTP %d)", code), "orion-agent token apply --token-file <file>   (replacement token from an admin)")
	case code >= 500:
		return warn(fmt.Sprintf("Core answered HTTP %d", code), "retry later; check Core health")
	}
	if e.Hostname != "" {
		return ok("valid, agent " + e.Hostname)
	}
	return ok("valid")
}

func checkLogAccess(_ context.Context, e *Env) Result {
	if e.Config == nil {
		return skip("no config yet")
	}
	path := e.Config.Logging.Path
	if path == "" || path == "stdout" || path == "stderr" || path == "-" || path == "none" {
		return skip("logging to the console")
	}
	can := e.CanRead
	if can == nil {
		can = func(p string) error {
			f, err := os.Open(p)
			if err == nil {
				f.Close()
			}
			return err
		}
	}
	err := can(path)
	switch {
	case err == nil:
		return ok("can read " + path)
	case os.IsNotExist(err):
		return warn(path+" does not exist yet", "sudo systemctl restart orion-agent   (the daemon creates it on start)")
	case os.IsPermission(err):
		return fail("cannot read "+path, "sudo usermod -aG orion $USER && sudo systemctl restart orion-agent   (or read through the daemon: orion-agent logs)")
	}
	return warn(firstLine(err.Error()), "")
}

// checkLogSources verifies that enabled product-log sources can obtain their
// credentials and reach the configured ingest endpoint. It deliberately does
// not send an entry: Doctor must not create product data as a side effect.
func checkLogSources(ctx context.Context, e *Env) Result {
	if e.Config == nil || len(e.Config.Logs.Sources) == 0 {
		return ok("no product log sources configured")
	}
	can := e.CanRead
	if can == nil {
		can = func(p string) error {
			f, err := os.Open(p)
			if err == nil {
				f.Close()
			}
			return err
		}
	}
	for _, source := range e.Config.Logs.Sources {
		if source.Type == "file" {
			if err := can(source.Path); err != nil && !os.IsNotExist(err) {
				return fail("source "+source.Name+": cannot read "+source.Path, "grant the orion user read access to "+source.Path)
			}
		}
		if source.IngestKeyFile != "" {
			if err := can(source.IngestKeyFile); err != nil {
				return fail("source "+source.Name+": cannot read ingest key", "sudo chgrp orion "+source.IngestKeyFile+" && sudo chmod 0640 "+source.IngestKeyFile)
			}
		}
		if source.IngestKeyEnv != "" && strings.TrimSpace(os.Getenv(source.IngestKeyEnv)) == "" {
			return fail("source "+source.Name+": "+source.IngestKeyEnv+" is not set", "set "+source.IngestKeyEnv+" for the orion-agent service and restart it")
		}
	}

	endpoint := e.Config.Logs.Endpoint
	if endpoint == "" {
		endpoint = strings.TrimRight(e.Config.CoreURL, "/") + "/v1/ingest/logs"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fail("invalid ingest endpoint", "set logs.endpoint to the Core /v1/ingest/logs URL")
	}
	client := e.HTTP
	if client == nil {
		client = &http.Client{Timeout: 6 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fail("cannot reach log ingest endpoint", "check logs.endpoint, DNS and firewall access to Core")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusNotFound {
		return fail(fmt.Sprintf("log ingest endpoint answered HTTP %d", resp.StatusCode), "check logs.endpoint and Core log ingest")
	}
	return ok(fmt.Sprintf("%d source%s, endpoint reachable", len(e.Config.Logs.Sources), plural(len(e.Config.Logs.Sources), "", "s")))
}

func checkDisk(_ context.Context, e *Env) Result {
	free := e.DiskFreeMB
	if free == nil {
		return skip("disk check unavailable")
	}
	dir := e.StatePath
	if i := strings.LastIndex(dir, "/"); i > 0 {
		dir = dir[:i]
	}
	mb, err := free(dir)
	if err != nil {
		return skip(firstLine(err.Error()))
	}
	msg := fmt.Sprintf("%s free", formatMB(mb))
	switch {
	case mb < 50:
		return fail(msg, "du -sh "+dir+" /var/log/orion   (free space: the spool and logs need room)")
	case mb < 500:
		return warn(msg, "du -sh "+dir+" /var/log/orion   (free space soon)")
	}
	return ok(msg)
}

func formatMB(mb uint64) string {
	if mb >= 1024 {
		return fmt.Sprintf("%d GB", mb/1024)
	}
	return fmt.Sprintf("%d MB", mb)
}

// checkSpool reports reports and logs waiting to be sent to Core.
func checkSpool(_ context.Context, e *Env) Result {
	if e.Queue == nil {
		return skip("daemon not reachable")
	}
	n, okay := e.Queue()
	if !okay {
		return skip("daemon not reachable")
	}
	if n > 100 {
		return warn(fmt.Sprintf("%d pending", n), "orion-agent status   (Core unreachable? the backlog drains when it returns)")
	}
	return ok(fmt.Sprintf("%d pending", n))
}
