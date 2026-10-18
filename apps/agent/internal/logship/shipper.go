package logship

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"orion/agent/internal/logging"
)

// SourceSpec is one configured log source.
type SourceSpec struct {
	Name      string
	Type      string // file | journald | docker
	Path      string // file glob
	Unit      string // journald unit
	Container string // docker container
	App       string // resource service.name; defaults to Name
	FromStart bool   // first run reads existing content instead of only new lines
	Multiline string // regex marking the start of an entry
	Redact    []string
	MinLevel  string
}

// Offset is a resumable position.
type Offset struct {
	FileID string
	Offset int64
	Cursor string
}

// OffsetStore persists where each source has been read up to.
type OffsetStore interface {
	Get(key string) (Offset, bool, error)
	Set(key string, o Offset) error
}

// Stats are cumulative counters, safe to read while running.
type Stats struct {
	Read, Spooled, Sent, Rejected, Dropped atomic.Int64
	LastError                              atomic.Value // string
}

// Shipper reads sources into the spool and drains the spool to Core.
type Shipper struct {
	Sources []SourceSpec
	Spool   *Spool
	Offsets OffsetStore
	Sender  Sender
	Host    string
	Now     func() time.Time

	BatchSize    int           // default 500
	FlushEvery   time.Duration // default 500ms
	GlobEvery    time.Duration // default 10s
	SendEvery    time.Duration // idle poll of the spool, default 1s
	MaxBackoff   time.Duration // default 60s
	TailPoll     time.Duration // default 250ms
	stats        Stats
	entries      chan Entry
	commits      chan commit
	sendNudge    chan struct{}
	sourceByName map[string]SourceSpec
}

type commit struct {
	key string
	off Offset
}

// Stats returns the live counters.
func (s *Shipper) Stats() *Stats { return &s.stats }

func (s *Shipper) defaults() {
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.BatchSize <= 0 {
		s.BatchSize = 500
	}
	if s.FlushEvery <= 0 {
		s.FlushEvery = 500 * time.Millisecond
	}
	if s.GlobEvery <= 0 {
		s.GlobEvery = 10 * time.Second
	}
	if s.SendEvery <= 0 {
		s.SendEvery = time.Second
	}
	if s.MaxBackoff <= 0 {
		s.MaxBackoff = time.Minute
	}
	if s.TailPoll <= 0 {
		s.TailPoll = 250 * time.Millisecond
	}
	s.entries = make(chan Entry, 4096)
	s.commits = make(chan commit, 4096)
	s.sendNudge = make(chan struct{}, 1)
	s.sourceByName = map[string]SourceSpec{}
	for _, sp := range s.Sources {
		s.sourceByName[sp.Name] = sp
	}
}

// Run blocks until ctx ends. On shutdown it flushes what it already read.
func (s *Shipper) Run(ctx context.Context) {
	s.defaults()
	var wg sync.WaitGroup
	readers := &sync.WaitGroup{}
	for _, sp := range s.Sources {
		sp := sp
		readers.Add(1)
		go func() { defer readers.Done(); s.runSource(ctx, sp) }()
	}
	wg.Add(2)
	go func() { defer wg.Done(); s.writeLoop(ctx) }()
	go func() { defer wg.Done(); s.sendLoop(ctx) }()
	readers.Wait()
	close(s.entries) // lets the writer drain and exit
	wg.Wait()
}

func (s *Shipper) runSource(ctx context.Context, sp SourceSpec) {
	switch sp.Type {
	case "file", "":
		s.runFileSource(ctx, sp)
	case "journald":
		s.runStream(ctx, sp, journaldCommand(sp, s.cursorFor(sp)))
	case "docker":
		s.runStream(ctx, sp, dockerCommand(sp, s.cursorFor(sp)))
	default:
		logging.Errorf("log source %s: unknown type %q", sp.Name, sp.Type)
	}
}

func (s *Shipper) cursorFor(sp SourceSpec) string {
	o, ok, _ := s.Offsets.Get(sp.Name)
	if !ok {
		return ""
	}
	return o.Cursor
}

// runFileSource expands the glob periodically and tails each match.
func (s *Shipper) runFileSource(ctx context.Context, sp SourceSpec) {
	redactor, err := NewRedactor(sp.Redact)
	if err != nil {
		logging.Errorf("log source %s: %v", sp.Name, err)
		return
	}
	ml, err := CompileMultiline(sp.Multiline)
	if err != nil {
		logging.Errorf("log source %s: invalid multiline regex: %v", sp.Name, err)
		return
	}
	active := map[string]context.CancelFunc{}
	var tw sync.WaitGroup
	defer func() {
		for _, cancel := range active {
			cancel()
		}
		tw.Wait()
	}()
	for {
		matches, _ := filepath.Glob(sp.Path)
		for _, path := range matches {
			if _, ok := active[path]; ok {
				continue
			}
			tctx, cancel := context.WithCancel(ctx)
			active[path] = cancel
			tw.Add(1)
			go func(path string) {
				defer tw.Done()
				s.tailFile(tctx, sp, path, redactor, ml)
			}(path)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.GlobEvery):
		}
	}
}

func (s *Shipper) tailFile(ctx context.Context, sp SourceSpec, path string, red *Redactor, ml interface{ MatchString(string) bool }) {
	key := sp.Name + ":" + path
	stored, _, _ := s.Offsets.Get(key)
	t := NewTailer(path, Position{FileID: stored.FileID, Offset: stored.Offset}, sp.FromStart)
	t.Poll = s.TailPoll
	if re, ok := ml.(interface{ MatchString(string) bool }); ok && ml != nil {
		_ = re
	}
	t.Multiline, _ = CompileMultiline(sp.Multiline)
	defer t.Close()
	t.Run(ctx, func(l Line) bool {
		e, ok := s.buildEntry(sp, path, red, l.Text, l.FileID, l.Start)
		if ok {
			select {
			case s.entries <- e:
			case <-ctx.Done():
				return false
			}
		}
		select {
		case s.commits <- commit{key: key, off: Offset{FileID: l.FileID, Offset: l.End}}:
		case <-ctx.Done():
			return false
		}
		return true
	})
}

var levelRank = map[string]int{"trace": 0, "debug": 1, "info": 2, "warn": 3, "error": 4, "fatal": 5}

func (s *Shipper) buildEntry(sp SourceSpec, path string, red *Redactor, text, fid string, start int64) (Entry, bool) {
	if strings.TrimSpace(text) == "" {
		return Entry{}, false
	}
	s.stats.Read.Add(1)
	body, sev, attrs, ts, event := parseLine(text)
	if sp.MinLevel != "" {
		if r, ok := levelRank[sev]; ok && r < levelRank[normalizeSeverity(sp.MinLevel)] {
			return Entry{}, false
		}
	}
	if attrs != nil {
		redactAttrs(attrs, red)
	}
	app := sp.App
	if app == "" {
		app = sp.Name
	}
	res := map[string]any{"service.name": app, "host.name": s.Host, "service.instance.id": s.Host, "log.source": sp.Name}
	if path != "" {
		res["log.file.path"] = path
	}
	if ts == "" {
		ts = nowRFC3339(s.Now)
	}
	return Entry{
		ID: entryID(sp.Name, path+"@"+fid, start), Timestamp: ts, Severity: sev, Body: red.Apply(body), Event: event,
		Attributes: attrs, Resource: res, Source: sp.Name,
	}, true
}

// writeLoop batches entries into the spool, then commits offsets: an offset is
// only advanced once the lines behind it are on disk.
func (s *Shipper) writeLoop(ctx context.Context) {
	pending := map[string]Offset{}
	var batch []Entry
	flush := func() {
		if len(batch) > 0 {
			if err := s.Spool.Append(batch); err != nil {
				logging.Errorf("log spool write failed: %v", err)
				s.stats.LastError.Store(err.Error())
				return // keep the batch and offsets; retry on the next tick
			}
			s.stats.Spooled.Add(int64(len(batch)))
			batch = batch[:0]
			select {
			case s.sendNudge <- struct{}{}:
			default:
			}
		}
		for k, o := range pending {
			if err := s.Offsets.Set(k, o); err != nil {
				logging.Warnf("could not save log offset for %s: %v", k, err)
			}
			delete(pending, k)
		}
	}
	tick := time.NewTicker(s.FlushEvery)
	defer tick.Stop()
	for {
		select {
		case e, ok := <-s.entries:
			if !ok {
				drainCommits(s.commits, pending)
				flush()
				return
			}
			batch = append(batch, e)
			drainCommits(s.commits, pending)
			if len(batch) >= s.BatchSize {
				flush()
			}
		case <-tick.C:
			drainCommits(s.commits, pending)
			flush()
		}
	}
}

func drainCommits(ch chan commit, pending map[string]Offset) {
	for {
		select {
		case c := <-ch:
			pending[c.key] = c.off
		default:
			return
		}
	}
}

// sendLoop drains the spool to Core with capped exponential backoff.
func (s *Shipper) sendLoop(ctx context.Context) {
	backoff := time.Second
	for {
		cur := s.Spool.Cursor()
		batch, next, err := s.Spool.ReadBatch(cur, s.BatchSize, 512<<10)
		if err == nil && len(batch) > 0 {
			if serr := s.sendBatch(ctx, batch); serr != nil {
				wait := backoff
				var ra *RetryAfterError
				if errors.As(serr, &ra) {
					wait = ra.After
				} else {
					backoff = min(backoff*2, s.MaxBackoff)
				}
				s.stats.LastError.Store(serr.Error())
				if !sleep(ctx, wait) {
					return
				}
				continue
			}
			backoff = time.Second
			_ = s.Spool.SaveCursor(next)
			continue
		}
		if next != cur {
			_ = s.Spool.SaveCursor(next)
		}
		select {
		case <-ctx.Done():
			return
		case <-s.sendNudge:
		case <-time.After(s.SendEvery):
		}
	}
}

// sendBatch sends each source's entries; a permanently rejected source is
// dropped with a counter, never blocking the others.
func (s *Shipper) sendBatch(ctx context.Context, batch []Entry) error {
	by := map[string][]Entry{}
	var order []string
	for _, e := range batch {
		if _, ok := by[e.Source]; !ok {
			order = append(order, e.Source)
		}
		by[e.Source] = append(by[e.Source], e)
	}
	for _, src := range order {
		err := s.Sender.Send(ctx, src, by[src])
		switch {
		case err == nil:
			s.stats.Sent.Add(int64(len(by[src])))
		case errors.Is(err, ErrRejected):
			s.stats.Rejected.Add(int64(len(by[src])))
			s.stats.LastError.Store(err.Error())
			logging.Errorf("log source %q: %v; %d entries dropped (fix the ingest key or payload)", src, err, len(by[src]))
		default:
			return err
		}
	}
	return nil
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// SpoolDir returns the default spool directory next to the state database.
func SpoolDir(statePath string) string {
	return filepath.Join(filepath.Dir(statePath), "logspool")
}

var _ = os.Getenv
