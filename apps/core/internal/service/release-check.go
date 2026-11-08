package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Release is one published Orion release.
type Release struct {
	// Version is the tag without a leading "v".
	Version    string
	ReleasedAt time.Time
	URL        string
	Prerelease bool
	// Highlights are the first bullet points of the release notes.
	Highlights []string
	// Security is set when the release notes carry the line "orion-security: true".
	Security bool
}

// ReleaseLookup finds the newest release on a channel ("stable" or "prerelease").
type ReleaseLookup interface {
	Latest(ctx context.Context, channel string) (Release, error)
}

// GitHubReleases reads the public releases of a repository. It sends no credentials.
type GitHubReleases struct {
	Repo    string
	APIBase string
	Client  *http.Client
}

func NewGitHubReleases(repo string) *GitHubReleases {
	return &GitHubReleases{Repo: repo, APIBase: "https://api.github.com", Client: &http.Client{Timeout: 10 * time.Second}}
}

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
}

const releaseBodyLimit = 2 << 20

// Latest returns the newest published release; the stable channel skips pre-releases.
func (g *GitHubReleases) Latest(ctx context.Context, channel string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(g.APIBase, "/")+"/repos/"+g.Repo+"/releases?per_page=15", nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "orion-core")
	resp, err := g.Client.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub answered %d", resp.StatusCode)
	}
	var list []githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, releaseBodyLimit)).Decode(&list); err != nil {
		return Release{}, errors.New("GitHub sent a release list Core could not read")
	}
	for _, item := range list {
		if item.Draft || (channel != "prerelease" && item.Prerelease) {
			continue
		}
		version := strings.TrimPrefix(strings.TrimSpace(item.TagName), "v")
		if _, ok := parseVersion(version); !ok {
			continue
		}
		return Release{Version: version, ReleasedAt: item.PublishedAt.UTC(), URL: item.HTMLURL, Prerelease: item.Prerelease, Security: hasSecurityMarker(item.Body), Highlights: releaseHighlights(item.Body)}, nil
	}
	return Release{}, errors.New("no release found")
}

// releaseHighlights returns up to five bullet points from the notes, shortened to one line each.
func releaseHighlights(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") && !strings.HasPrefix(line, "* ") {
			continue
		}
		text := strings.TrimSpace(line[2:])
		if r := []rune(text); len(r) > 120 {
			text = string(r[:119]) + "…"
		}
		if text != "" {
			out = append(out, text)
		}
		if len(out) == 5 {
			break
		}
	}
	return out
}

func hasSecurityMarker(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.EqualFold(strings.TrimSpace(line), "orion-security: true") {
			return true
		}
	}
	return false
}

type parsedVersion struct {
	nums []int
	pre  string
}

func parseVersion(v string) (parsedVersion, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	core, pre, _ := strings.Cut(v, "-")
	core, _, _ = strings.Cut(core, "+")
	parts := strings.Split(core, ".")
	if core == "" || len(parts) > 3 {
		return parsedVersion{}, false
	}
	out := parsedVersion{pre: pre}
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return parsedVersion{}, false
		}
		out.nums = append(out.nums, n)
	}
	for len(out.nums) < 3 {
		out.nums = append(out.nums, 0)
	}
	return out, true
}

// CompareVersions orders two versions. ok is false when either is not a version, such as a "dev" build.
func CompareVersions(a, b string) (cmp int, ok bool) {
	pa, okA := parseVersion(a)
	pb, okB := parseVersion(b)
	if !okA || !okB {
		return 0, false
	}
	for i := range pa.nums {
		if pa.nums[i] != pb.nums[i] {
			if pa.nums[i] < pb.nums[i] {
				return -1, true
			}
			return 1, true
		}
	}
	switch {
	case pa.pre == pb.pre:
		return 0, true
	case pa.pre == "":
		return 1, true
	case pb.pre == "":
		return -1, true
	case pa.pre < pb.pre:
		return -1, true
	}
	return 1, true
}
