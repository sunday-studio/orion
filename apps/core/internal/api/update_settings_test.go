package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"orion/core/internal/buildinfo"
	"orion/core/internal/service"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type fakeReleases struct {
	release service.Release
	err     error
	calls   int
	channel string
}

func (f *fakeReleases) Latest(_ context.Context, channel string) (service.Release, error) {
	f.calls++
	f.channel = channel
	return f.release, f.err
}

func TestUpdateSettingsLifecycle(t *testing.T) {
	previous := buildinfo.Version
	buildinfo.Version = "0.9.2"
	t.Cleanup(func() { buildinfo.Version = previous })
	server, jwt := authedServer(t)
	fake := &fakeReleases{release: service.Release{Version: "0.9.3", ReleasedAt: time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC), URL: "https://example.test/093", Highlights: []string{"Faster startup"}}}
	server.releases = fake
	read := func(method, path string, body any) UpdateSettings {
		t.Helper()
		var out struct {
			Data UpdateSettings `json:"data"`
		}
		r := bearer(t, server, method, path, jwt, body)
		if r.Code != http.StatusOK {
			t.Fatalf("%s %s = %d %s", method, path, r.Code, r.Body.String())
		}
		_ = json.Unmarshal(r.Body.Bytes(), &out)
		return out.Data
	}

	if got := read(http.MethodGet, "/v1/settings/updates", nil); got.State != "unchecked" || !got.AutoCheck || got.Channel != "stable" || got.CurrentVersion != "0.9.2" {
		t.Fatalf("defaults = %+v", got)
	}
	got := read(http.MethodPost, "/v1/settings/updates/check", nil)
	if got.State != "available" || got.LatestVersion != "0.9.3" || got.CheckedAt == nil || got.ReleaseURL == "" || len(got.Highlights) != 1 || fake.channel != "stable" {
		t.Fatalf("after check = %+v (channel %q)", got, fake.channel)
	}

	if r := bearer(t, server, http.MethodPut, "/v1/settings/updates", jwt, gin.H{"skipped_version": "0.9.9"}); r.Code != http.StatusBadRequest {
		t.Errorf("skipping a version that was not found = %d, want 400", r.Code)
	}
	if got := read(http.MethodPut, "/v1/settings/updates", gin.H{"skipped_version": "0.9.3"}); got.State != "skipped" {
		t.Fatalf("after skip = %+v", got)
	}
	if got := read(http.MethodPut, "/v1/settings/updates", gin.H{"skipped_version": ""}); got.State != "available" {
		t.Fatalf("after undoing the skip = %+v", got)
	}

	// A security release cannot be skipped away.
	fake.release.Security = true
	read(http.MethodPost, "/v1/settings/updates/check", nil)
	if got := read(http.MethodPut, "/v1/settings/updates", gin.H{"skipped_version": "0.9.3"}); got.State != "security" {
		t.Fatalf("security release after a skip = %+v", got)
	}

	// Changing the channel forgets what was found.
	if got := read(http.MethodPut, "/v1/settings/updates", gin.H{"channel": "prerelease"}); got.State != "unchecked" || got.LatestVersion != "" || got.SkippedVersion != "" {
		t.Fatalf("after channel change = %+v", got)
	}
	if r := bearer(t, server, http.MethodPut, "/v1/settings/updates", jwt, gin.H{"channel": "nightly"}); r.Code != http.StatusBadRequest {
		t.Errorf("bad channel = %d, want 400", r.Code)
	}

	// A failed check is an answer, not an HTTP error, and keeps the last known release out of the way.
	fake.err = errors.New("could not reach GitHub")
	if got := read(http.MethodPost, "/v1/settings/updates/check", nil); got.State != "error" || got.Error == "" {
		t.Fatalf("failed check = %+v", got)
	}
	fake.err = nil
	fake.release = service.Release{Version: "0.9.2"}
	read(http.MethodPost, "/v1/settings/updates/check", nil)
	if got := read(http.MethodGet, "/v1/settings/updates", nil); got.State != "up_to_date" || got.Error != "" {
		t.Fatalf("on the latest = %+v", got)
	}
	if got := read(http.MethodPut, "/v1/settings/updates", gin.H{"auto_check": false}); got.State != "off" || got.AutoCheck {
		t.Fatalf("checks off = %+v", got)
	}
}

func TestUpdateSettingsDevBuildNeverClaimsAnUpdate(t *testing.T) {
	previous := buildinfo.Version
	buildinfo.Version = "dev"
	t.Cleanup(func() { buildinfo.Version = previous })
	server, jwt := authedServer(t)
	server.releases = &fakeReleases{release: service.Release{Version: "0.9.3"}}
	var out struct {
		Data UpdateSettings `json:"data"`
	}
	_ = json.Unmarshal(bearer(t, server, http.MethodPost, "/v1/settings/updates/check", jwt, nil).Body.Bytes(), &out)
	if out.Data.State != "up_to_date" || out.Data.LatestVersion != "0.9.3" {
		t.Fatalf("dev build = %+v", out.Data)
	}
}
