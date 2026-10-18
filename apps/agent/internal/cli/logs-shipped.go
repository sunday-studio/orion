package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"orion/agent/internal/logship"
)

type shippedOpts struct {
	Follow bool
	Level  string
	Name   string
	Search string
	Lines  int
}

// fetchSpool reads the shipped-log spool: through the daemon's socket (works
// for any user in the orion group), else directly when the files are readable.
func fetchSpool(opts *Options, n int) ([]logship.Entry, error) {
	data, err := controlClient(opts).Spool(n)
	if err != nil {
		sp, serr := logship.OpenSpool(logship.SpoolDir(opts.StatePath), 0)
		if serr != nil {
			return nil, err
		}
		defer sp.Close()
		return sp.Recent(n)
	}
	var out []logship.Entry
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		var e logship.Entry
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			out = append(out, e)
		}
	}
	return out, nil
}

// runShippedLogs shows logs shipped from this host (plan E4: offline, from the
// local spool). Follow is interactive on a terminal and a plain tail otherwise.
func runShippedLogs(ctx context.Context, opts *Options, o shippedOpts) error {
	level := o.Level
	if level == "" {
		level = "info"
	}
	if norm := normalizeSeverity(level); norm != "" {
		level = norm
	}
	fetch := func() ([]logship.Entry, error) { return fetchSpool(opts, 2000) }
	entries, err := fetch()
	if err != nil {
		return NewCommandError("could not read the log spool", err, "is log shipping configured? add logs.sources to the config (orion-agent config edit)", "or show the agent's own log: orion-agent logs --source file")
	}
	st := logsState{Entries: entries, Level: level, Source: o.Name, Search: o.Search}
	lines := o.Lines
	if lines <= 0 {
		lines = defaultLogLines
	}

	if opts.JSON {
		for _, e := range st.visibleOldestFirst(lines) {
			b, _ := json.Marshal(e)
			fmt.Fprintln(jsonWriter(), string(b))
		}
		return nil
	}
	if !o.Follow {
		fmt.Fprint(outputWriter, logsScreen(theme, st, terminalWidth(), lines))
		return nil
	}
	if !theme.Plain() && stdoutIsTerminal() {
		return runLogsTUI(ctx, theme, fetch, level, o.Name)
	}
	return tailPlain(ctx, opts, st, lines)
}

func normalizeSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace", "debug", "info", "warn", "error", "fatal":
		return strings.ToLower(strings.TrimSpace(s))
	case "warning":
		return "warn"
	}
	return ""
}

func (s logsState) visibleOldestFirst(n int) []logship.Entry {
	v := s.visible(n)
	for i, j := 0, len(v)-1; i < j; i, j = i+1, j-1 {
		v[i], v[j] = v[j], v[i]
	}
	return v
}

// tailPlain prints matching entries oldest first, then new ones as they arrive.
func tailPlain(ctx context.Context, opts *Options, st logsState, lines int) error {
	seen := map[string]bool{}
	print := func(es []logship.Entry) {
		cur := st
		cur.Entries = es
		for _, e := range cur.visibleOldestFirst(len(es)) {
			if seen[e.ID] {
				continue
			}
			seen[e.ID] = true
			sev := e.Severity
			if sev == "" {
				sev = "info"
			}
			fmt.Fprintf(outputWriter, "%s %-5s %-7s %s\n", clock(e.Timestamp), sev, clip(entrySource(e), 7), strings.SplitN(e.Body, "\n", 2)[0])
		}
	}
	first := st.visibleOldestFirst(lines)
	print(first)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second):
		}
		if es, err := fetchSpool(opts, 2000); err == nil {
			print(es)
		}
	}
}

// jsonWriter is where --json output goes (stdout; human output is discarded).
func jsonWriter() io.Writer {
	if jsonOut != nil {
		return jsonOut
	}
	return os.Stdout
}
