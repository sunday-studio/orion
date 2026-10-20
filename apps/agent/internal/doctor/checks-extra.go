package doctor

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// checkState opens the state database read-only.
func checkState(_ context.Context, e *Env) Result {
	if e.StateOpen == nil {
		return skip("cannot check from here")
	}
	if err := e.StateOpen(); err != nil {
		return fail(firstLine(err.Error()), "sudo orion-agent state init   (or check who owns "+e.StatePath+")").
			titled("The state database cannot be opened", firstLine(err.Error()))
	}
	return ok(e.StatePath).titled("State database")
}

// checkDiscovery reports the daemon's latest scan.
func checkDiscovery(_ context.Context, e *Env) Result {
	if e.Discovery == nil {
		return skip("daemon not reachable")
	}
	d, reachable := e.Discovery()
	if !reachable {
		return skip("daemon not reachable")
	}
	if d.ScannedAt.IsZero() {
		return warn("no scan yet", "orion-agent discover --rescan").titled("The host has not been scanned yet")
	}
	when := d.ScannedAt.Local().Format("15:04")
	if e.now().Sub(d.ScannedAt) > 24*time.Hour {
		when = d.ScannedAt.Local().Format("2 Jan 15:04")
	}
	return ok(fmt.Sprintf("%s · found %d %s", when, d.Things, plural(d.Things, "thing", "things"))).titled("Last scan")
}

// checkDiscoveryAccess warns about kinds the agent could not read.
func checkDiscoveryAccess(_ context.Context, e *Env) Result {
	if e.Discovery == nil {
		return skip("daemon not reachable")
	}
	d, reachable := e.Discovery()
	if !reachable || len(d.Blocked) == 0 {
		return skip("nothing blocked")
	}
	b := d.Blocked[0]
	r := warn(b.Message, b.Fix).titled(b.Message)
	if b.Why != "" {
		r.Evidence = []string{b.Why}
	}
	if len(d.Blocked) > 1 {
		r.Evidence = append(r.Evidence, fmt.Sprintf("%d more kinds are not fully scanned: orion-agent discover", len(d.Blocked)-1))
	}
	return r
}

// checkLogKeys warns when a log source's key file is empty.
func checkLogKeys(_ context.Context, e *Env) Result {
	if e.Config == nil || len(e.Config.Logs.Sources) == 0 {
		return skip("no log sources")
	}
	read := e.ReadKey
	if read == nil {
		read = func(p string) (string, error) { b, err := os.ReadFile(p); return string(b), err }
	}
	n := 0
	for _, src := range e.Config.Logs.Sources {
		if src.IngestKeyFile == "" {
			continue
		}
		n++
		key, err := read(src.IngestKeyFile)
		if err != nil {
			continue // checkLogSources reports an unreadable key file
		}
		if strings.TrimSpace(key) == "" {
			return warn("key file is empty", "paste the key from Console › Logs › Apps into "+src.IngestKeyFile+"   (then orion-agent restart)").
				titled("Log key for "+src.Name+" is missing", src.IngestKeyFile+" is empty")
		}
	}
	return ok(fmt.Sprintf("%d key file%s", n, plural(n, "", "s"))).titled("Log keys")
}
