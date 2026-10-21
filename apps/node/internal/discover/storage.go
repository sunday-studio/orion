package discover

import (
	"context"
	"fmt"
	"path"
	"strings"

	"orion/node/internal/config"
)

// KindStorage is a directory whose size is worth tracking for an app.
const KindStorage Kind = "storage"

// wellKnownPaths maps well-known app names to where they keep data or logs.
var wellKnownPaths = map[string][]string{
	"postgresql": {"/var/lib/postgresql"}, "postgres": {"/var/lib/postgresql"},
	"mysql": {"/var/lib/mysql"}, "mysqld": {"/var/lib/mysql"}, "mariadb": {"/var/lib/mysql"},
	"mongod": {"/var/lib/mongodb"}, "redis": {"/var/lib/redis"}, "redis-server": {"/var/lib/redis"},
	"elasticsearch": {"/var/lib/elasticsearch"}, "rabbitmq-server": {"/var/lib/rabbitmq"},
	"nginx": {"/var/log/nginx"}, "caddy": {"/var/lib/caddy", "/var/log/caddy"},
	"grafana-server": {"/var/lib/grafana"}, "prometheus": {"/var/lib/prometheus"},
}

// storageRoots are the places where app data and logs normally live. Anything
// else a unit declares (a working directory of "/", /usr, /etc) is not "the
// app's space" and is not proposed.
var storageRoots = []string{"/var/lib/", "/var/log/", "/var/cache/", "/srv/", "/opt/", "/data/", "/home/"}

func wanted(p string) bool {
	if !path.IsAbs(p) || p != path.Clean(p) {
		return false
	}
	for _, r := range storageRoots {
		if strings.HasPrefix(p, r) && len(p) > len(r) {
			if r == "/home/" && strings.Count(strings.TrimPrefix(p, r), "/") < 1 {
				return false // a whole home directory is not "the app's space"
			}
			return true
		}
	}
	return false
}

// storagePaths proposes tracking the space used by the apps already found:
// directories their systemd units declare, plus well-known data directories.
func storagePaths(ctx context.Context, e Env, found []Candidate) ([]Candidate, []string) {
	var notes []string
	type hit struct {
		app, dir, why string
		conf          Confidence
	}
	var hits []hit

	var units []string
	for _, c := range found {
		if c.Kind == KindService && c.Monitor != nil && c.Monitor.Systemd != nil {
			units = append(units, c.Monitor.Systemd.Name+".service")
		}
		if c.Kind == KindService || c.Kind == KindPort {
			name := strings.ToLower(strings.SplitN(strings.TrimPrefix(c.Name, ":"), "@", 2)[0])
			for _, p := range wellKnownPaths[name] {
				hits = append(hits, hit{name, p, "well-known data directory", High})
			}
		}
	}
	if len(units) > 0 && e.GOOS == "linux" {
		if len(units) > 40 {
			units = units[:40]
		}
		args := append([]string{"show", "--no-pager", "-p", "Id", "-p", "WorkingDirectory", "-p", "StateDirectory", "-p", "LogsDirectory", "-p", "CacheDirectory"}, units...)
		if out, err := e.Run(ctx, "systemctl", args...); err != nil {
			notes = append(notes, fmt.Sprintf("storage: systemctl show: %v", err))
		} else {
			for _, block := range strings.Split(string(out), "\n\n") {
				props := map[string]string{}
				for _, line := range strings.Split(block, "\n") {
					if k, v, ok := strings.Cut(line, "="); ok {
						props[k] = strings.TrimSpace(v)
					}
				}
				app := strings.TrimSuffix(props["Id"], ".service")
				if app == "" {
					continue
				}
				if wd := props["WorkingDirectory"]; wanted(wd) {
					hits = append(hits, hit{app, wd, "its working directory", Medium})
				}
				for key, root := range map[string]string{"StateDirectory": "/var/lib/", "LogsDirectory": "/var/log/", "CacheDirectory": "/var/cache/"} {
					for _, d := range strings.Fields(props[key]) {
						if p := path.Join(root, d); wanted(p) {
							hits = append(hits, hit{app, p, "its " + strings.ToLower(strings.TrimSuffix(key, "Directory")) + " directory", Medium})
						}
					}
				}
			}
		}
	}

	var out []Candidate
	seen := map[string]bool{}
	for _, h := range hits {
		if seen[h.dir] || e.Exists == nil || !e.Exists(h.dir) {
			continue
		}
		seen[h.dir] = true
		conf := h.conf
		detail := "space used by " + h.app + " (" + h.why + ")"
		if e.Readable != nil && !e.Readable(h.dir) {
			conf, detail = Low, detail+"; the node cannot read it yet"
			notes = append(notes, fmt.Sprintf("storage: no read access to %s; grant the node user read access to track %s", h.dir, h.app))
		}
		out = append(out, Candidate{ID: "storage:" + h.dir, Kind: KindStorage, Name: h.dir, Detail: detail, Confidence: conf, StoragePath: h.dir})
	}
	return out, notes
}

// AddStoragePath appends p to the tracked paths once.
func AddStoragePath(cfg *config.UserConfig, p string) {
	for _, have := range cfg.AppResources.Paths {
		if have == p {
			return
		}
	}
	cfg.AppResources.Paths = append(cfg.AppResources.Paths, p)
}
