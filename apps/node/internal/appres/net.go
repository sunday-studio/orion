package appres

import (
	"context"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// hostNetwork reads per-process network counters where an unprivileged,
// supported source exists (macOS `nettop`). On Linux there is none (see the
// support matrix), so it returns nothing and no note.
func hostNetwork() (map[string]NetCounters, error) {
	if runtime.GOOS != "darwin" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nettop", "-P", "-L", "1", "-J", "bytes_in,bytes_out").Output()
	if err != nil {
		return nil, err
	}
	return parseNettop(string(out)), nil
}

// parseNettop parses `nettop -P -L 1 -J bytes_in,bytes_out`: a header line
// then "name.pid,bytes_in,bytes_out," rows. Rows are summed by name.
func parseNettop(out string) map[string]NetCounters {
	res := map[string]NetCounters{}
	for i, line := range strings.Split(out, "\n") {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, ",")
		if len(f) < 3 {
			continue
		}
		name := f[0]
		if dot := strings.LastIndex(name, "."); dot > 0 {
			if _, err := strconv.Atoi(name[dot+1:]); err == nil {
				name = name[:dot]
			}
		}
		rx, e1 := strconv.ParseUint(strings.TrimSpace(f[1]), 10, 64)
		tx, e2 := strconv.ParseUint(strings.TrimSpace(f[2]), 10, 64)
		if e1 != nil || e2 != nil {
			continue
		}
		c := res[name]
		c.Rx += rx
		c.Tx += tx
		res[name] = c
	}
	return res
}
