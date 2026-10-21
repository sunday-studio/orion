package discover

import (
	"context"
	"errors"
	"strings"
	"testing"

	"orion/node/internal/config"

	"gopkg.in/yaml.v3"
)

func fixtureEnv() Env {
	outputs := map[string]string{
		"systemctl": "nginx.service loaded active running A high performance web server\n" +
			"systemd-journald.service loaded active running Journal\n" +
			"dbus.service loaded active running D-Bus\n" +
			"myapp.service loaded active running My App\n" +
			"postgresql@16-main.service loaded active running PostgreSQL\n",
		"docker": `{"Names":"web","Image":"nginx:1.27","Status":"Up 2 hours"}` + "\n" + `{"Names":"cache","Image":"redis:7","Status":"Up 2 hours"}` + "\n",
		"pm2":    `[PM2] banner` + "\n" + `[{"name":"api","pm2_env":{"status":"online"}}]`,
	}
	return Env{
		GOOS: "linux", Interval: "60s",
		Run: func(_ context.Context, name string, _ ...string) ([]byte, error) {
			if o, ok := outputs[name]; ok {
				return []byte(o), nil
			}
			return nil, errors.New("not found")
		},
		Ports: func(context.Context) ([]ListeningPort, error) {
			return []ListeningPort{{80, "nginx"}, {22, "sshd"}, {3000, "node"}, {5432, "postgres"}, {9999, ""}}, nil
		},
		Glob: func(p string) ([]string, error) {
			if p == "/var/log/nginx/error.log" || p == "/var/log/syslog" {
				return []string{p}, nil
			}
			return nil, nil
		},
		LookupBin: func(string) bool { return true },
	}
}

func ids(res Result) []string {
	var out []string
	for _, c := range res.Candidates {
		out = append(out, c.ID+"="+string(c.Confidence))
	}
	return out
}

func TestFixtureHostDiscovery(t *testing.T) {
	res := Run(context.Background(), fixtureEnv())
	got := strings.Join(ids(res), ",")
	for _, want := range []string{
		"service:nginx=high", "service:postgresql@16-main=high", "service:myapp=medium",
		"container:web=high", "container:cache=high", "pm2:api=high",
		"port:80=high", "port:3000=medium", "port:9999=low", "log:/var/log/nginx/error.log=medium", "log:/var/log/syslog=low",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
	for _, unwanted := range []string{"systemd-journald", "dbus", "port:22"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("noise %s should be filtered: %s", unwanted, got)
		}
	}
	// High confidence comes first.
	if res.Candidates[0].Confidence != High {
		t.Errorf("first candidate = %+v, want high confidence first", res.Candidates[0])
	}
}

func TestFailingSourceAddsNoteAndKeepsGoing(t *testing.T) {
	e := fixtureEnv()
	base := e.Run
	e.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "docker" {
			return nil, errors.New("permission denied")
		}
		return base(ctx, name, args...)
	}
	res := Run(context.Background(), e)
	if len(res.Notes) != 1 || !strings.Contains(res.Notes[0], "docker") {
		t.Fatalf("notes = %v", res.Notes)
	}
	if !strings.Contains(strings.Join(ids(res), ","), "service:nginx") {
		t.Fatal("other sources must still report")
	}
}

// The wizard's output equals the hand-written config for the fixture host.
func TestBuildConfigEqualsHandWrittenConfig(t *testing.T) {
	res := Run(context.Background(), fixtureEnv())
	sel, err := Select(res.Candidates, []string{"service:nginx", "container:web", "port:3000"})
	if err != nil {
		t.Fatal(err)
	}
	cfg := BuildConfig(config.UserConfig{CoreURL: "https://core.example.com", Interval: "60s", Monitors: []config.UserMonitor{}}, sel)
	got, _ := yaml.Marshal(cfg)

	hand := `core_url: https://core.example.com
interval: 60s
monitors:
    - name: svc-nginx
      description: systemd unit nginx
      type: systemd-service
      interval: 60s
      meta:
        source: scan
      systemd:
        name: nginx
    - name: ctr-web
      description: docker container web
      type: docker-container
      interval: 60s
      meta:
        source: scan
      docker:
        name: web
    - name: port-3000
      description: TCP port 3000 (node)
      type: tcp
      interval: 60s
      meta:
        source: scan
      tcp:
        host: 127.0.0.1
        port: 3000
        timeout: 3s
`
	var a, b config.UserConfig
	yaml.Unmarshal(got, &a)
	if err := yaml.Unmarshal([]byte(hand), &b); err != nil {
		t.Fatal(err)
	}
	ga, _ := yaml.Marshal(a)
	gb, _ := yaml.Marshal(b)
	if string(ga) != string(gb) {
		t.Fatalf("generated config differs from hand-written:\n%s\nvs\n%s", ga, gb)
	}
	b.ApplyDefaults()
	if err := b.Validate(); err != nil {
		t.Fatalf("generated config must validate: %v", err)
	}
}

func TestSelectUnknownIDExplainsFix(t *testing.T) {
	if _, err := Select(nil, []string{"service:ghost"}); err == nil || !strings.Contains(err.Error(), "orion-node discover") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildConfigKeepsExistingMonitors(t *testing.T) {
	res := Run(context.Background(), fixtureEnv())
	sel, _ := Select(res.Candidates, []string{"service:nginx"})
	base := config.UserConfig{Monitors: []config.UserMonitor{{Name: "svc-nginx", Description: "hand edited"}}}
	out := BuildConfig(base, sel)
	if len(out.Monitors) != 1 || out.Monitors[0].Description != "hand edited" {
		t.Fatalf("existing monitor was overwritten: %+v", out.Monitors)
	}
}

func storageEnv() Env {
	e := fixtureEnv()
	base := e.Run
	e.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "systemctl" && len(args) > 0 && args[0] == "show" {
			return []byte("Id=myapp.service\nWorkingDirectory=/srv/myapp\nStateDirectory=myapp\nLogsDirectory=\nCacheDirectory=\n\n" +
				"Id=nginx.service\nWorkingDirectory=/\nStateDirectory=\nLogsDirectory=nginx\nCacheDirectory=\n\n" +
				"Id=postgresql@16-main.service\nWorkingDirectory=/usr/lib/postgresql\nStateDirectory=\nLogsDirectory=\nCacheDirectory=\n"), nil
		}
		return base(ctx, name, args...)
	}
	have := map[string]bool{"/srv/myapp": true, "/var/lib/myapp": true, "/var/log/nginx": true, "/var/lib/postgresql": true, "/var/lib/redis": true}
	e.Exists = func(d string) bool { return have[d] }
	e.Readable = func(d string) bool { return d != "/var/lib/postgresql" }
	return e
}

func TestStorageSuggestionsFromUnitsAndWellKnownPaths(t *testing.T) {
	res := Run(context.Background(), storageEnv())
	by := map[string]Candidate{}
	for _, c := range res.Candidates {
		by[c.ID] = c
	}
	for id, conf := range map[string]Confidence{
		"storage:/srv/myapp": Medium, "storage:/var/lib/myapp": Medium, "storage:/var/log/nginx": High,
	} {
		if c, ok := by[id]; !ok || c.Confidence != conf || c.StoragePath == "" || c.Monitor != nil {
			t.Errorf("%s = %+v (ok=%v)", id, c, ok)
		}
	}
	if _, ok := by["storage:/usr/lib/postgresql"]; ok {
		t.Error("/usr paths are not an app's data space")
	}
	if _, ok := by["storage:/"]; ok {
		t.Error("a working directory of / must not be proposed")
	}
	if _, ok := by["storage:/var/lib/redis"]; ok {
		t.Error("redis is not running in this fixture, so its path must not be proposed")
	}
	pg := by["storage:/var/lib/postgresql"]
	if pg.Confidence != Low || !strings.Contains(pg.Detail, "cannot read") {
		t.Errorf("unreadable data dir must be low confidence with a reason: %+v", pg)
	}
	found := false
	for _, n := range res.Notes {
		if strings.Contains(n, "no read access to /var/lib/postgresql") {
			found = true
		}
	}
	if !found {
		t.Errorf("missing read-access note: %v", res.Notes)
	}
}

func TestSelectedStorageBecomesTrackedPathsWithoutDuplicates(t *testing.T) {
	res := Run(context.Background(), storageEnv())
	sel, err := Select(res.Candidates, []string{"storage:/srv/myapp", "storage:/var/log/nginx"})
	if err != nil {
		t.Fatal(err)
	}
	base := config.UserConfig{AppResources: config.AppResourcesConfig{Paths: []string{"/srv/myapp"}}}
	out := BuildConfig(base, sel)
	if strings.Join(out.AppResources.Paths, ",") != "/srv/myapp,/var/log/nginx" {
		t.Fatalf("paths = %v", out.AppResources.Paths)
	}
	checked := out
	checked.CoreURL, checked.Interval = "https://x", "60s"
	if err := checked.Validate(); err != nil {
		t.Fatalf("result must validate: %v", err)
	}
}

func TestWholeHomeDirectoryIsNotProposed(t *testing.T) {
	if wanted("/home/alice") || !wanted("/home/alice/app") || wanted("/home/") || wanted("/") || wanted("/usr/lib/x") {
		t.Fatal("home-directory depth rule broken")
	}
}
