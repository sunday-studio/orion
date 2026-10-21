package cli

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"orion/node/internal/config"
	"orion/node/internal/logging"
	"orion/node/internal/logship"
	nodestate "orion/node/internal/state"
)

// stateOffsets stores log read positions in the node state database.
type stateOffsets struct{ s *nodestate.Store }

func (o stateOffsets) Get(key string) (logship.Offset, bool, error) {
	v, ok, err := o.s.GetLogOffset(key)
	return logship.Offset{FileID: v.FileID, Offset: v.Offset, Cursor: v.Cursor}, ok, err
}

func (o stateOffsets) Set(key string, off logship.Offset) error {
	return o.s.SetLogOffset(key, nodestate.LogOffset{FileID: off.FileID, Offset: off.Offset, Cursor: off.Cursor})
}

// keyResolver reads each source's ingest key from its file or env var and
// re-reads it periodically, so rotating a key needs no restart.
type keyResolver struct {
	cfg  []config.LogSource
	mu   sync.Mutex
	seen map[string]cachedKey
}

type cachedKey struct {
	key string
	at  time.Time
}

func (k *keyResolver) Key(source string) string {
	k.mu.Lock()
	defer k.mu.Unlock()
	if c, ok := k.seen[source]; ok && time.Since(c.at) < 30*time.Second {
		return c.key
	}
	key := ""
	for _, s := range k.cfg {
		if s.Name != source {
			continue
		}
		switch {
		case s.IngestKeyEnv != "":
			key = strings.TrimSpace(os.Getenv(s.IngestKeyEnv))
		case s.IngestKeyFile != "":
			if b, err := os.ReadFile(s.IngestKeyFile); err == nil {
				key = strings.TrimSpace(string(b))
			}
		}
	}
	if k.seen == nil {
		k.seen = map[string]cachedKey{}
	}
	k.seen[source] = cachedKey{key, time.Now()}
	return key
}

func toSpecs(srcs []config.LogSource) []logship.SourceSpec {
	out := make([]logship.SourceSpec, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, logship.SourceSpec{
			Name: s.Name, Type: s.Type, Path: s.Path, Unit: s.Unit, Container: s.Container, App: s.App,
			FromStart: s.From == "beginning", Multiline: s.MultilineStart, Redact: s.Redact, MinLevel: s.MinLevel,
		})
	}
	return out
}

// startLogShipper runs the configured sources until ctx ends. It returns a
// wait function; with no sources it does nothing.
func startLogShipper(ctx context.Context, opts *Options, cfg *config.UserConfig, store *nodestate.Store, ctrl *daemonControl) (wait func()) {
	ctrl.logSources.Store(int64(len(cfg.Logs.Sources)))
	if len(cfg.Logs.Sources) == 0 {
		ctrl.setSpool(nil)
		return func() {}
	}
	spool, err := logship.OpenSpool(logship.SpoolDir(opts.StatePath), cfg.Logs.SpoolMB)
	if err != nil {
		logging.Errorf("Log shipping disabled: %v", err)
		return func() {}
	}
	ctrl.setSpool(spool)
	endpoint := cfg.Logs.Endpoint
	if endpoint == "" {
		endpoint = strings.TrimRight(cfg.CoreURL, "/") + "/v1/ingest/logs"
	}
	host, _ := os.Hostname()
	sh := &logship.Shipper{
		Sources: toSpecs(cfg.Logs.Sources), Spool: spool, Offsets: stateOffsets{store}, Host: host,
		Sender: &logship.HTTPSender{URL: endpoint, Key: (&keyResolver{cfg: cfg.Logs.Sources}).Key},
	}
	done := make(chan struct{})
	go func() { defer close(done); sh.Run(ctx); spool.Close() }()
	logging.Infof("Log shipper started: %d source(s) -> %s", len(cfg.Logs.Sources), endpoint)
	return func() { <-done }
}
