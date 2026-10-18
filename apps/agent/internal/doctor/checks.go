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

// DefaultChecks is the standard check list, in dependency order.
func DefaultChecks() []Check {
	return []Check{
		{"config", "Configuration is valid", checkConfig},
		{"service", "Service is installed and running", checkService},
		{"daemon", "Daemon answers on its control socket", checkDaemon},
		{"version", "CLI and daemon versions match", checkVersion},
		{"dns", "Core hostname resolves", checkDNS},
		{"tls", "TLS certificate for Core is valid", checkTLS},
		{"core", "Core is reachable", checkCore},
		{"clock", "Clock is in sync with Core", checkClock},
		{"token", "Agent token is accepted by Core", checkToken},
		{"log-access", "Configured log files are readable", checkLogAccess},
		{"disk", "State directory has free space", checkDisk},
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

func checkConfig(_ context.Context, e *Env) Result {
	if e.ConfigErr != nil {
		return fail(e.ConfigErr.Error(), "fix the line above, then run: orion-agent config validate  (or create one: orion-agent setup --core-url https://your-core)")
	}
	return ok(fmt.Sprintf("%s, %d monitor(s)", e.ConfigPath, len(e.Config.Monitors)))
}

func checkService(_ context.Context, e *Env) Result {
	if e.Service == nil {
		return skip("no service manager detected")
	}
	installed, running, state := e.Service()
	switch {
	case !installed:
		return fail("service is not installed", "install it: curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- --core-url <url>")
	case !running:
		start := "sudo systemctl start orion-agent"
		if e.GOOS == "darwin" {
			start = "sudo launchctl bootstrap system /Library/LaunchDaemons/com.orion.agent.plist"
		}
		return fail("service state: "+state, start+"   (then see why it stopped: orion-agent logs)")
	}
	return ok("running")
}

func checkDaemon(_ context.Context, e *Env) Result {
	if e.Daemon.Reachable {
		return ok("pid reachable at " + e.SocketPath)
	}
	msg := "not reachable"
	if e.Daemon.Err != nil {
		msg = e.Daemon.Err.Error()
	}
	if strings.Contains(msg, "permission") {
		return warn(msg, "sudo usermod -aG orion $USER  (log out and back in), or run this command with sudo")
	}
	return warn(msg, "start the service (see the service check); for a manual run use: orion-agent run")
}

func checkVersion(_ context.Context, e *Env) Result {
	if !e.Daemon.Reachable {
		return skip("daemon not reachable")
	}
	if e.Daemon.Version != e.Version {
		return warn(fmt.Sprintf("cli %s, daemon %s", e.Version, e.Daemon.Version), "restart the service to run the new binary: orion-agent restart")
	}
	return ok(e.Version)
}

func checkDNS(ctx context.Context, e *Env) Result {
	u, good := coreURL(e)
	if !good {
		return skip("no valid core_url")
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
		return fail(fmt.Sprintf("cannot resolve %s: %v", host, err), "check DNS: cat /etc/resolv.conf  and  getent hosts "+host+"  (or use the Core's IP address in core_url)")
	}
	return ok(fmt.Sprintf("%s -> %s", host, addrs[0]))
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
		return fail(err.Error(), "if Core uses a private CA, install it: sudo cp ca.crt /usr/local/share/ca-certificates/ && sudo update-ca-certificates  (macOS: sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ca.crt)")
	}
	left := notAfter.Sub(e.now())
	switch {
	case left < 0:
		return fail("certificate expired "+notAfter.Format("2006-01-02"), "renew the certificate on the Core host")
	case left < 14*24*time.Hour:
		return warn(fmt.Sprintf("certificate expires in %d day(s)", int(left.Hours()/24)), "renew the certificate on the Core host")
	}
	return ok("valid until " + notAfter.Format("2006-01-02"))
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

func (e *Env) get(ctx context.Context, path, bearer string) (*http.Response, error) {
	u, _ := coreURL(e)
	base := strings.TrimRight(e.Config.CoreURL, "/")
	_ = u
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return nil, err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	client := e.HTTP
	if client == nil {
		client = &http.Client{Timeout: 6 * time.Second}
	}
	return client.Do(req)
}

func checkCore(ctx context.Context, e *Env) Result {
	if _, good := coreURL(e); !good {
		return skip("no valid core_url")
	}
	resp, err := e.get(ctx, "/health", "")
	if err != nil {
		return fail(err.Error(), "check the network path to Core: curl -v "+strings.TrimRight(e.Config.CoreURL, "/")+"/health  (firewall, proxy, or wrong core_url?)")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fail(fmt.Sprintf("Core answered HTTP %d", resp.StatusCode), "check Core's own health and logs on the Core host")
	}
	return ok(fmt.Sprintf("HTTP %d", resp.StatusCode))
}

func checkClock(ctx context.Context, e *Env) Result {
	if _, good := coreURL(e); !good {
		return skip("no valid core_url")
	}
	resp, err := e.get(ctx, "/health", "")
	if err != nil {
		return skip("Core not reachable")
	}
	defer resp.Body.Close()
	date, perr := http.ParseTime(resp.Header.Get("Date"))
	if perr != nil {
		return skip("Core did not send a Date header")
	}
	skew := e.now().Sub(date)
	if skew < 0 {
		skew = -skew
	}
	fix := "sudo timedatectl set-ntp true   (macOS: sudo sntp -sS time.apple.com)"
	switch {
	case skew > 5*time.Minute:
		return fail(fmt.Sprintf("clock differs from Core by %s", skew.Round(time.Second)), fix)
	case skew > time.Minute:
		return warn(fmt.Sprintf("clock differs from Core by %s", skew.Round(time.Second)), fix)
	}
	return ok(fmt.Sprintf("skew %s", skew.Round(time.Second)))
}

func checkToken(ctx context.Context, e *Env) Result {
	if !e.Registered {
		return warn("this host is not enrolled yet", "enroll it with the command from Console > Add server  (orion-agent enroll --core-url <url> --token-file <file>)")
	}
	if e.TokenProbe == nil {
		return skip("token cannot be checked from here; run doctor as a user in the orion group")
	}
	code, err := e.TokenProbe(ctx)
	if err != nil {
		return skip("Core not reachable: " + err.Error())
	}
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return fail(fmt.Sprintf("Core rejected the agent token (HTTP %d)", code), "apply a replacement token from an admin: orion-agent token apply --token-file <file>")
	case code >= 500:
		return warn(fmt.Sprintf("Core answered HTTP %d", code), "retry later; check Core health")
	}
	return ok("accepted")
}

func checkLogAccess(_ context.Context, e *Env) Result {
	if e.Config == nil {
		return skip("config unavailable")
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
		return ok(path)
	case os.IsNotExist(err):
		return warn(path+" does not exist yet", "it is created when the daemon starts: sudo systemctl restart orion-agent")
	case os.IsPermission(err):
		return warn("cannot read "+path, "sudo usermod -aG orion $USER  (then log out and in), or read logs through the daemon: orion-agent logs")
	}
	return warn(err.Error(), "")
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
		return skip(err.Error())
	}
	switch {
	case mb < 50:
		return fail(fmt.Sprintf("%d MB free in %s", mb, dir), "free disk space: the agent spool and logs need room (du -sh "+dir+" /var/log/orion)")
	case mb < 500:
		return warn(fmt.Sprintf("%d MB free in %s", mb, dir), "free disk space soon")
	}
	return ok(fmt.Sprintf("%d MB free", mb))
}
