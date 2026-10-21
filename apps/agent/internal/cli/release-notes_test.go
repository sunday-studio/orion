package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"orion/agent/internal/ui"
)

func TestSanitizeMarkdownDropsTerminalControlSequences(t *testing.T) {
	got := sanitizeMarkdown("# Hi\x1b[2J\x1b]0;pwned\x07\r\n- item\there\x7f")
	if strings.ContainsAny(got, "\x1b\x07\r\x7f") || !strings.Contains(got, "# Hi") || !strings.Contains(got, "item\there") {
		t.Fatalf("got %q", got)
	}
}

func TestPlainMarkdownIsPrintedAsWrittenAndColorIsFormatted(t *testing.T) {
	md := "# Title\n\n- **bold** item\n"
	if got := renderMarkdown(ui.Plain(), md, 80); got != md {
		t.Fatalf("plain = %q", got)
	}
	out := renderMarkdown(ui.WithMode(true), md, 80)
	if strings.Contains(out, "# Title") || strings.Contains(out, "**bold**") || !strings.Contains(out, "Title") || !strings.Contains(out, "bold") {
		t.Fatalf("formatted = %q", out)
	}
}

func TestFetchReleaseNotesLatestTagAndFailures(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		switch {
		case strings.HasSuffix(path, "/tags/v0.0"):
			http.NotFound(w, r)
		case strings.HasSuffix(path, "/tags/v0.1"):
			w.Write([]byte(`{"tag_name":"v0.1","body":"  "}`))
		default:
			w.Write([]byte(`{"tag_name":"v1.2.0","body":"## Fixed\n- thing"}`))
		}
	}))
	defer srv.Close()
	old := releaseNotesAPI
	releaseNotesAPI = srv.URL
	defer func() { releaseNotesAPI = old }()

	n, err := fetchReleaseNotes("o/r", "latest")
	if err != nil || n.Tag != "v1.2.0" || !strings.Contains(n.Body, "Fixed") || path != "/repos/o/r/releases/latest" {
		t.Fatalf("latest = %+v %v path=%s", n, err, path)
	}
	if _, err := fetchReleaseNotes("o/r", "v0.0"); err == nil {
		t.Fatal("a missing release is an error the caller ignores")
	}
	if _, err := fetchReleaseNotes("o/r", "v0.1"); err == nil {
		t.Fatal("empty notes are not shown")
	}
	if n, _ := fetchReleaseNotes("o/r", "v1.2.0"); n == nil || path != "/repos/o/r/releases/tags/v1.2.0" {
		t.Fatalf("a pinned version asks for its tag: %s", path)
	}
}

func TestHelpTopicsPrintAndUnknownOnesFallThrough(t *testing.T) {
	refreshTheme()
	SetColorEnabled(false)
	var b bytes.Buffer
	if !printHelpTopic(&b, "exit-codes", 80) || !strings.Contains(b.String(), "needs elevation") {
		t.Fatalf("exit-codes = %q", b.String())
	}
	b.Reset()
	if !printHelpTopic(&b, "topics", 80) || !strings.Contains(b.String(), "discovery") || !strings.Contains(b.String(), "output-modes") {
		t.Fatalf("topics = %q", b.String())
	}
	if printHelpTopic(&b, "doctor", 80) {
		t.Fatal("a command name is not a topic: its own help must run")
	}
}
