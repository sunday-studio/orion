package discover

import (
	"context"
	"errors"
	"strings"
	"testing"

	"orion/node/internal/config"
)

func envWithMounts() Env {
	e := fixtureEnv()
	e.Mounts = func() ([]Mount, error) {
		return []Mount{
			{Path: "/", Device: "/dev/sda1", FSType: "ext4", UsedPercent: 44, TotalBytes: 50 << 30},
			{Path: "/var", Device: "/dev/sdb1", FSType: "ext4", UsedPercent: 91, TotalBytes: 100 << 30},
			{Path: "/boot", Device: "/dev/sda2", FSType: "ext4", UsedPercent: 20, TotalBytes: 1 << 30},
			{Path: "/var/lib/docker/overlay", Device: "overlay", FSType: "ext4", UsedPercent: 10, TotalBytes: 50 << 30},
			{Path: "/mnt/tiny", Device: "/dev/sdc1", FSType: "ext4", UsedPercent: 5, TotalBytes: 100 << 20},
		}, nil
	}
	return e
}

func byID(r Result) map[string]Candidate {
	m := map[string]Candidate{}
	for _, c := range r.Candidates {
		m[c.ID] = c
	}
	return m
}

func TestDiskSuggestionsSkipBootAndOverlaysAndMarkTinyMedium(t *testing.T) {
	r := byID(Run(context.Background(), envWithMounts()))
	if c := r["disk:/var"]; c.Confidence != High || c.Monitor == nil || c.Monitor.DiskUsage.MaxPercent != 90 || !strings.Contains(c.Detail, "91% used") {
		t.Fatalf("disk:/var = %+v", c)
	}
	if r["disk:/mnt/tiny"].Confidence != Medium {
		t.Fatalf("a tiny filesystem should be medium: %+v", r["disk:/mnt/tiny"])
	}
	for _, id := range []string{"disk:/boot", "disk:/var/lib/docker/overlay"} {
		if _, ok := r[id]; ok {
			t.Errorf("%s must not be proposed", id)
		}
	}
}

func TestKindsOffAreNotScannedAndReportedOff(t *testing.T) {
	e := envWithMounts()
	called := false
	base := e.Run
	e.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "docker" {
			called = true
		}
		return base(ctx, name, args...)
	}
	e.Kinds = map[string]bool{ConfigServices: true, ConfigDisks: true} // everything else off
	res := Run(context.Background(), e)
	if called {
		t.Fatal("a kind that is off must not be touched at all")
	}
	for _, c := range res.Candidates {
		if k := ConfigKind(c.Kind); k != ConfigServices && k != ConfigDisks {
			t.Errorf("candidate from a disabled kind: %+v", c)
		}
	}
	states := map[string]string{}
	for _, k := range res.Kinds {
		states[k.Kind] = k.State
	}
	if states[ConfigContainers] != ScanOff || states[ConfigServices] != ScanOK {
		t.Fatalf("kinds = %v", states)
	}
}

func TestBlockedDockerNamesFixAndTradeOff(t *testing.T) {
	e := fixtureEnv()
	base := e.Run
	e.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "docker" {
			return nil, errors.New("docker ps (is the service user in the docker group?): permission denied")
		}
		return base(ctx, name, args...)
	}
	var k KindStatus
	for _, s := range Run(context.Background(), e).Kinds {
		if s.Kind == ConfigContainers {
			k = s
		}
	}
	if k.State != ScanBlocked || !strings.Contains(k.Message, "Could not look at Docker") || !strings.Contains(k.Fix, "usermod -aG docker orion") || !strings.Contains(k.Why, "powerful") {
		t.Fatalf("blocked docker = %+v", k)
	}
}

func TestIgnoredAreNeverSuggestedAndHandWrittenWins(t *testing.T) {
	e := envWithMounts()
	e.Ignore = []string{"service:myapp", "port:3000"}
	e.Existing = []config.UserMonitor{{Name: "my-nginx-check", Type: config.UserMonitorTypeSystemd, Systemd: &config.SystemdServiceConfig{Name: "nginx"}},
		{Name: "api", Type: config.UserMonitorTypeDocker, Docker: &config.DockerContainerConfig{Name: "web"}, Meta: map[string]interface{}{"source": "scan"}}}
	e.Tracked = []string{"/var/log/nginx"}
	e.Exists = func(d string) bool { return d == "/var/log/nginx" }
	e.Readable = func(string) bool { return true }
	r := byID(Run(context.Background(), e))
	if _, ok := r["service:myapp"]; ok {
		t.Error("an ignored id must never be suggested")
	}
	if _, ok := r["port:3000"]; ok {
		t.Error("an ignored port must never be suggested")
	}
	if c := r["service:nginx"]; c.Status != StatusMonitored || c.ManagedIn != "file" {
		t.Fatalf("a hand-written monitor wins: %+v", c)
	}
	if c := r["container:web"]; c.Status != StatusMonitored || c.ManagedIn != "scan" {
		t.Fatalf("an accepted monitor shows its origin: %+v", c)
	}
	if c := r["storage:/var/log/nginx"]; c.Status != StatusMonitored {
		t.Fatalf("an already tracked path: %+v", c)
	}
	if c := r["service:postgresql@16-main"]; c.Status != StatusSuggested {
		t.Fatalf("others stay suggested: %+v", c)
	}
}

func TestMonitoredSortLast(t *testing.T) {
	e := fixtureEnv()
	e.Existing = []config.UserMonitor{{Name: "n", Type: config.UserMonitorTypeSystemd, Systemd: &config.SystemdServiceConfig{Name: "nginx"}}}
	res := Run(context.Background(), e)
	seenMonitored := false
	for _, c := range res.Candidates {
		if c.Status == StatusMonitored {
			seenMonitored = true
		} else if seenMonitored {
			t.Fatalf("suggestions must come before monitored rows: %v", res.Candidates)
		}
	}
}
