package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
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
	// ReleaseTag returns the tag GitHub lists for a version, exactly as written ("0.3.0" or "v0.3.0").
	ReleaseTag(ctx context.Context, version string) (string, error)
	// Checksums returns the SHA-256 of each file in a release, by file name, as published in its
	// SHA256SUMS. tag is the release tag, for example "v0.9.3".
	Checksums(ctx context.Context, tag string) (map[string]string, error)
}

// GitHubReleases reads the public releases of a repository. It sends no credentials.
type GitHubReleases struct {
	Repo    string
	APIBase string
	// DownloadBase replaces https://github.com/<repo>/releases/download in tests.
	DownloadBase string
	Client       *http.Client
}

var (
	releaseTagPattern = regexp.MustCompile(`^v?\d{1,4}\.\d{1,4}\.\d{1,4}(-[0-9A-Za-z.]{1,32})?$`)
	releaseSumPattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

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

// ReleaseTag finds the tag of a published release by its version. Tags are not all spelled the same
// way ("0.3.0", "v0.3.0"), and the download address needs the one GitHub has.
func (g *GitHubReleases) ReleaseTag(ctx context.Context, version string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(g.APIBase, "/")+"/repos/"+g.Repo+"/releases?per_page=100", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "orion-core")
	resp, err := g.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub answered %d", resp.StatusCode)
	}
	var list []githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, releaseBodyLimit)).Decode(&list); err != nil {
		return "", errors.New("GitHub sent a release list Core could not read")
	}
	for _, item := range list {
		tag := strings.TrimSpace(item.TagName)
		if !item.Draft && releaseTagPattern.MatchString(tag) && strings.TrimPrefix(tag, "v") == version {
			return tag, nil
		}
	}
	return "", fmt.Errorf("GitHub has no published release %s", version)
}

// Checksums reads the SHA256SUMS file of one release.
func (g *GitHubReleases) Checksums(ctx context.Context, tag string) (map[string]string, error) {
	if !releaseTagPattern.MatchString(tag) {
		return nil, fmt.Errorf("%q is not a release tag", tag)
	}
	url := "https://github.com/" + g.Repo + "/releases/download/" + tag + "/SHA256SUMS"
	if g.DownloadBase != "" {
		url = strings.TrimRight(g.DownloadBase, "/") + "/" + tag + "/SHA256SUMS"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "orion-core")
	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the release has no SHA256SUMS (GitHub answered %d)", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && releaseSumPattern.MatchString(f[0]) {
			out[strings.TrimPrefix(f[1], "*")] = strings.ToLower(f[0])
		}
	}
	return out, nil
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
