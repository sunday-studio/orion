package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"orion/node/internal/ui"
)

func TestSanitizeMarkdownDropsTerminalControlSequences(t *testing.T) {
	got := sanitizeMarkdown("# Hi\x1b[2J\x1b]0;pwned\x07\r\n- item\there\x7f\u202e evil\u2028")
	if strings.ContainsAny(got, "\x1b\x07\r\x7f\u202e\u2028") || !strings.Contains(got, "# Hi") || !strings.Contains(got, "item\there") {
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

func TestMiniMarkdownStylesWhatWeUseAndLeavesTheRestAlone(t *testing.T) {
	md := "# Title\n\n## Section\n\nSome **bold** and `code` and [docs](https://x.io/a).\n\n- first\n- second item\n\n1. one\n\n```\nraw  code\n```\n\n| a | b |\n"
	out := renderMarkdown(ui.WithMode(true), md, 80)
	for _, gone := range []string{"# Title", "## Section", "**bold**", "`code`", "[docs]", "```"} {
		if strings.Contains(out, gone) {
			t.Errorf("markup %q should be rendered, got:\n%s", gone, out)
		}
	}
	for _, want := range []string{"Title", "Section", "bold", "code", "docs (https://x.io/a)", "•", "first", "1.", "raw  code", "| a | b |"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if out := renderMarkdown(ui.WithMode(true), "#123 fixed a crash\n", 80); !strings.Contains(out, "#123 fixed a crash") {
		t.Errorf("an issue reference is text, not a heading: %q", out)
	}
	long := renderMarkdown(ui.WithMode(true), strings.Repeat("word ", 60), 60)
	for _, l := range strings.Split(strings.TrimRight(long, "\n"), "\n") {
		if visibleWidth(l) > 60 {
			t.Errorf("line wider than the terminal: %d", visibleWidth(l))
		}
	}
}

func TestUpdateIsRefusedInsideAContainerBeforeAskingForPrivileges(t *testing.T) {
	t.Setenv("ORION_CONTAINER", "1")
	if err := refuseUpdateInContainer(); err == nil || !strings.Contains(err.Error(), "container") {
		t.Fatalf("err = %v", err)
	}
	t.Setenv("ORION_CONTAINER", "")
	if err := refuseUpdateInContainer(); err != nil {
		t.Fatalf("outside a container update runs: %v", err)
	}
}

func TestGlobalFlagsDoNotHideHelpTopicsAndTransposedTyposAreSuggested(t *testing.T) {
	got := withoutGlobalFlags([]string{"--config", "/x/c.yaml", "--state=/x/s.db", "help", "--no-color", "topics"})
	if len(got) != 2 || got[0] != "help" || got[1] != "topics" {
		t.Fatalf("got %v", got)
	}
	if w := closestWord("statsu", []string{"state", "status", "start"}); w != "status" {
		t.Fatalf("statsu -> %q, want status (a swap is one edit)", w)
	}
	if w := closestWord("zzzzzz", []string{"state", "status"}); w != "" {
		t.Fatalf("nothing is close to zzzzzz, got %q", w)
	}
}
