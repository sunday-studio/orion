package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
		ok   bool
	}{
		{"0.9.2", "0.9.3", -1, true},
		{"v0.9.3", "0.9.3", 0, true},
		{"0.10.0", "0.9.9", 1, true},
		{"1.0", "1.0.0", 0, true},
		{"0.9.3-rc1", "0.9.3", -1, true},
		{"0.9.3", "0.9.3-rc1", 1, true},
		{"dev", "0.9.3", 0, false},
		{"0.9.x", "0.9.3", 0, false},
	}
	for _, c := range cases {
		got, ok := CompareVersions(c.a, c.b)
		if got != c.want || ok != c.ok {
			t.Errorf("CompareVersions(%q, %q) = %d, %v; want %d, %v", c.a, c.b, got, ok, c.want, c.ok)
		}
	}
}

func TestGitHubReleasesPicksByChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/sunday-studio/orion/releases" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`[
		 {"tag_name":"v0.9.4-rc1","prerelease":true,"html_url":"https://x/rc","published_at":"2026-10-25T10:00:00Z","body":""},
		 {"tag_name":"v0.9.9","draft":true,"html_url":"https://x/draft","published_at":"2026-10-24T10:00:00Z"},
		 {"tag_name":"v0.9.3","html_url":"https://x/093","published_at":"2026-10-21T08:00:00Z","body":"Fixes.\n\n- Faster startup\n* Fixes a stuck queue\nOrion-Security: true\n"},
		 {"tag_name":"v0.9.2","html_url":"https://x/092","published_at":"2026-10-03T08:00:00Z","body":""}]`))
	}))
	defer server.Close()
	g := &GitHubReleases{Repo: "sunday-studio/orion", APIBase: server.URL, Client: server.Client()}
	stable, err := g.Latest(context.Background(), "stable")
	if err != nil || stable.Version != "0.9.3" || !stable.Security || stable.URL != "https://x/093" || len(stable.Highlights) != 2 || stable.Highlights[0] != "Faster startup" {
		t.Fatalf("stable = %+v, %v", stable, err)
	}
	pre, err := g.Latest(context.Background(), "prerelease")
	if err != nil || pre.Version != "0.9.4-rc1" || !pre.Prerelease || pre.Security {
		t.Fatalf("prerelease = %+v, %v", pre, err)
	}
}

func TestGitHubReleasesReportsFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer server.Close()
	g := &GitHubReleases{Repo: "x/y", APIBase: server.URL, Client: server.Client()}
	if _, err := g.Latest(context.Background(), "stable"); err == nil {
		t.Fatal("want an error for a 403")
	}
}
