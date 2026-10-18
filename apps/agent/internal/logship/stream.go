package logship

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"orion/agent/internal/logging"
)

// commandFactory builds the subprocess for a stream source; tests replace it.
var commandFactory = func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

type streamCmd struct {
	name string
	args []string
	// parse converts one output line into body, severity, timestamp and the
	// cursor that resumes after it.
	parse func(line string) (body, severity, ts, cursor string, ok bool)
}

func journaldCommand(sp SourceSpec, cursor string) streamCmd {
	args := []string{"-u", sp.Unit, "-o", "json", "--follow", "--no-pager"}
	switch {
	case cursor != "":
		args = append(args, "--after-cursor", cursor)
	case sp.FromStart:
		args = append(args, "--since", "-1h")
	default:
		args = append(args, "-n", "0")
	}
	return streamCmd{name: "journalctl", args: args, parse: parseJournald}
}

func parseJournald(line string) (body, severity, ts, cursor string, ok bool) {
	var m map[string]any
	if json.Unmarshal([]byte(line), &m) != nil {
		return
	}
	body, _ = m["MESSAGE"].(string)
	cursor, _ = m["__CURSOR"].(string)
	if body == "" || cursor == "" {
		return "", "", "", "", false
	}
	if p, ok2 := m["PRIORITY"].(string); ok2 {
		switch n, _ := strconv.Atoi(p); {
		case n <= 2:
			severity = "fatal"
		case n == 3:
			severity = "error"
		case n == 4:
			severity = "warn"
		case n <= 6:
			severity = "info"
		default:
			severity = "debug"
		}
	}
	if us, ok2 := m["__REALTIME_TIMESTAMP"].(string); ok2 {
		if v, err := strconv.ParseInt(us, 10, 64); err == nil {
			ts = time.UnixMicro(v).UTC().Format(time.RFC3339Nano)
		}
	}
	return body, severity, ts, cursor, true
}

func dockerCommand(sp SourceSpec, cursor string) streamCmd {
	args := []string{"logs", "--follow", "--timestamps"}
	switch {
	case cursor != "":
		args = append(args, "--since", cursor)
	case sp.FromStart:
	default:
		args = append(args, "--since", time.Now().UTC().Format(time.RFC3339Nano))
	}
	args = append(args, sp.Container)
	return streamCmd{name: "docker", args: args, parse: func(line string) (string, string, string, string, bool) {
		ts, rest, found := strings.Cut(line, " ")
		if !found || rest == "" {
			return "", "", "", "", false
		}
		return rest, "", ts, ts, true
	}}
}

// runStream follows a subprocess and restarts it (from the saved cursor) if it exits.
func (s *Shipper) runStream(ctx context.Context, sp SourceSpec, first streamCmd) {
	red, err := NewRedactor(sp.Redact)
	if err != nil {
		logging.Errorf("log source %s: %v", sp.Name, err)
		return
	}
	cmdSpec := first
	for {
		cmd := commandFactory(ctx, cmdSpec.name, cmdSpec.args...)
		out, err := cmd.StdoutPipe()
		if err == nil {
			err = cmd.Start()
		}
		if err != nil {
			logging.Errorf("log source %s: cannot start %s: %v", sp.Name, cmdSpec.name, err)
			s.stats.LastError.Store(fmt.Sprintf("%s: %v", sp.Name, err))
		} else {
			sc := bufio.NewScanner(out)
			sc.Buffer(make([]byte, 64*1024), MaxLineBytes*2)
			var seq int64
			for sc.Scan() {
				body, sev, ts, cursor, ok := cmdSpec.parse(sc.Text())
				if !ok {
					continue
				}
				seq++
				e, keep := s.buildEntry(sp, "", red, body, cursor, seq)
				if keep {
					if sev != "" {
						e.Severity = sev
					}
					if ts != "" {
						e.Timestamp = ts
					}
					e.ID = entryID(sp.Name, "stream", int64(hash(cursor)))
					select {
					case s.entries <- e:
					case <-ctx.Done():
						_ = cmd.Wait()
						return
					}
				}
				select {
				case s.commits <- commit{key: sp.Name, off: Offset{Cursor: cursor}}:
				case <-ctx.Done():
					_ = cmd.Wait()
					return
				}
			}
			_ = cmd.Wait()
		}
		if !sleep(ctx, 5*time.Second) {
			return
		}
		switch sp.Type {
		case "journald":
			cmdSpec = journaldCommand(sp, s.latestCursor(sp))
		case "docker":
			cmdSpec = dockerCommand(sp, s.latestCursor(sp))
		}
	}
}

func (s *Shipper) latestCursor(sp SourceSpec) string { return s.cursorFor(sp) }

func hash(s string) uint64 {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}
