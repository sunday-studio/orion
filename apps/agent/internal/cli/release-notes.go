package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	releaseNotesTimeout = 3 * time.Second
	releaseNotesMaxSize = 64 << 10
)

// releaseNotesAPI is overridden in tests.
var releaseNotesAPI = "https://api.github.com"

// releaseNotes is the published description of a release.
type releaseNotes struct {
	Tag  string
	Body string
}

// fetchReleaseNotes asks GitHub for the notes of version ("latest" or a tag).
// It is best effort: any failure returns an error the caller ignores, because
// the update must never depend on it. The text is display-only and never executed.
func fetchReleaseNotes(repo, version string) (*releaseNotes, error) {
	path := "/releases/latest"
	if version != "" && version != "latest" {
		path = "/releases/tags/" + version
	}
	req, err := http.NewRequest(http.MethodGet, releaseNotesAPI+"/repos/"+repo+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := (&http.Client{Timeout: releaseNotesTimeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release notes: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Tag  string `json:"tag_name"`
		Body string `json:"body"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, releaseNotesMaxSize)).Decode(&out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.Body) == "" {
		return nil, fmt.Errorf("release notes: empty")
	}
	return &releaseNotes{Tag: out.Tag, Body: out.Body}, nil
}

// releaseNotesView is the "What's new" block shown before an update starts.
func releaseNotesView(n *releaseNotes, current string, width int) string {
	title := "What's new in " + n.Tag
	if current != "" {
		title += " (you have " + current + ")"
	}
	return "\n " + theme.Accent("◆") + " " + theme.Bold(title) + "\n\n" + renderMarkdown(theme, n.Body, width) + "\n"
}
