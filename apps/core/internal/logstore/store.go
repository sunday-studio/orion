package logstore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

// ErrQueueFull means the in-process write queue is at capacity: the caller should answer 429.
var ErrQueueFull = errors.New("log ingest queue is full")

// ErrStorageFull means the last write failed because the disk or database is full. Enqueue
// refuses new entries for storageFullBackoff (answer 503 + Retry-After) instead of accepting
// data it cannot persist, then lets one batch probe whether space has returned.
var ErrStorageFull = errors.New("log storage is full")

var storageFullBackoff = 5 * time.Second // var so tests can shorten it

const (
	DefaultQueueMax      = 50_000
	writerBatchEntries   = 1000
	writerBatchWait      = 50 * time.Millisecond
	insertChunk          = 400
	logsDatabaseFileName = "logs.db"
)

const schema = `
CREATE TABLE IF NOT EXISTS log_entries (
  id TEXT PRIMARY KEY,
  app_id TEXT NOT NULL,
  client_id TEXT NOT NULL DEFAULT '',
  ts INTEGER NOT NULL,
  ingested_at INTEGER NOT NULL,
  severity TEXT NOT NULL,
  severity_number INTEGER NOT NULL,
  event TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL DEFAULT '',
  body_json TEXT NOT NULL DEFAULT '',
  attributes TEXT NOT NULL DEFAULT '',
  resource TEXT NOT NULL DEFAULT '',
  trace_id TEXT NOT NULL DEFAULT '',
  span_id TEXT NOT NULL DEFAULT '',
  request_id TEXT NOT NULL DEFAULT '',
  size INTEGER NOT NULL DEFAULT 0,
  severity_inferred INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_log_entries_app_ts ON log_entries(app_id, ts DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_log_entries_app_sev_ts ON log_entries(app_id, severity_number, ts DESC);
CREATE INDEX IF NOT EXISTS idx_log_entries_app_event_ts ON log_entries(app_id, event, ts DESC) WHERE event <> '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_log_entries_app_client_id ON log_entries(app_id, client_id) WHERE client_id <> '';
CREATE INDEX IF NOT EXISTS idx_log_entries_trace ON log_entries(trace_id) WHERE trace_id <> '';
CREATE INDEX IF NOT EXISTS idx_log_entries_request ON log_entries(request_id) WHERE request_id <> '';
CREATE INDEX IF NOT EXISTS idx_log_entries_ts ON log_entries(ts);
`

// Stats are cumulative since process start.
type Stats struct {
	Queued  int64 `json:"queued"`
	Written int64 `json:"written"`
	Dropped int64 `json:"dropped"`
	// Duplicates are entries ignored because their client id was already stored.
	Duplicates int64 `json:"duplicates"`
	// StorageFull is true while the last write failed with a disk-full error.
	StorageFull bool `json:"storage_full"`
}

// Store owns logs.db, the bounded queue in front of it and the single writer.
type Store struct {
	db       *gorm.DB
	logger   *utils.Logger
	queueMax int64

	batches chan []Entry
	queued  atomic.Int64
	written atomic.Int64
	dropped atomic.Int64
	dupes   atomic.Int64
	// fullAt is the unix-nano time of the last disk-full write failure (0 = healthy).
	fullAt atomic.Int64

	// sendMu orders Enqueue's send against Close: senders hold the read lock, Close the write lock.
	sendMu sync.RWMutex

	hub *hub

	usageMu    sync.Mutex
	usageAt    time.Time
	usageCache Usage

	closeOnce sync.Once
	closed    atomic.Bool
	done      chan struct{}
}

// Open opens (creating if needed) logs.db in dataDir, applies the schema and starts the writer.
func Open(dataDir string, logger *utils.Logger, queueMax int) (*Store, error) {
	database, err := db.OpenFile(dataDir, logsDatabaseFileName, "warn")
	if err != nil {
		return nil, err
	}
	return OpenWith(database, logger, queueMax)
}

// OpenReader opens logs.db for queries only (no writer goroutine). The Worker uses it
// to evaluate log-based alerts; the API process remains the only writer.
func OpenReader(dataDir string, logger *utils.Logger) (*Store, error) {
	database, err := db.OpenFile(dataDir, logsDatabaseFileName, "warn")
	if err != nil {
		return nil, err
	}
	if err := applySchema(database); err != nil {
		return nil, err
	}
	s := &Store{db: database, logger: logger, queueMax: 1, batches: make(chan []Entry), done: make(chan struct{}), hub: newHub()}
	close(s.done)
	s.closed.Store(true)
	return s, nil
}

// OpenWith starts a Store on an existing handle (tests).
func OpenWith(database *gorm.DB, logger *utils.Logger, queueMax int) (*Store, error) {
	if err := applySchema(database); err != nil {
		return nil, err
	}
	if queueMax <= 0 {
		queueMax = DefaultQueueMax
	}
	s := &Store{db: database, logger: logger, queueMax: int64(queueMax), batches: make(chan []Entry, 4096), done: make(chan struct{}), hub: newHub()}
	go s.writer()
	return s, nil
}

// DB exposes the logs handle for queries and diagnostics.
func (s *Store) DB() *gorm.DB { return s.db }

func (s *Store) Stats() Stats {
	return Stats{Queued: s.queued.Load(), Written: s.written.Load(), Dropped: s.dropped.Load(), Duplicates: s.dupes.Load(), StorageFull: s.fullAt.Load() != 0}
}

// Enqueue accepts a batch for writing without blocking. It fails with ErrQueueFull
// when the queue would exceed its entry budget, so a slow disk turns into 429s
// instead of unbounded memory.
func (s *Store) Enqueue(entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}
	s.sendMu.RLock()
	defer s.sendMu.RUnlock()
	if s.closed.Load() {
		return ErrQueueFull
	}
	if at := s.fullAt.Load(); at != 0 && time.Since(time.Unix(0, at)) < storageFullBackoff {
		return ErrStorageFull
	}
	n := int64(len(entries))
	if s.queued.Add(n) > s.queueMax {
		s.queued.Add(-n)
		return ErrQueueFull
	}
	select {
	case s.batches <- entries:
		return nil
	default:
		s.queued.Add(-n)
		return ErrQueueFull
	}
}

// Flush blocks until everything enqueued so far has been written (or the timeout passes).
func (s *Store) Flush(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for s.queued.Load() > 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
}

// Close stops accepting entries, drains the queue and waits for the writer.
func (s *Store) Close() {
	s.closeOnce.Do(func() {
		s.sendMu.Lock() // waits for in-flight Enqueue calls, then blocks new ones
		wasClosed := s.closed.Swap(true)
		if !wasClosed {
			close(s.batches)
		}
		s.sendMu.Unlock()
		<-s.done
		if sqlDB, err := s.db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
}

func (s *Store) writer() {
	defer close(s.done)
	for batch := range s.batches {
		pending := batch
		timer := time.NewTimer(writerBatchWait)
	collect:
		for len(pending) < writerBatchEntries {
			select {
			case more, ok := <-s.batches:
				if !ok {
					break collect
				}
				pending = append(pending, more...)
			case <-timer.C:
				break collect
			}
		}
		timer.Stop()
		s.flushBatch(pending)
	}
}

func (s *Store) flushBatch(entries []Entry) {
	defer s.queued.Add(-int64(len(entries)))
	stored, err := s.insert(entries)
	if err != nil {
		// One retry covers a transient lock; after that the batch is counted as dropped and logged.
		time.Sleep(100 * time.Millisecond)
		if stored, err = s.insert(entries); err != nil {
			s.dropped.Add(int64(len(entries)))
			if isStorageFull(err) {
				s.fullAt.Store(time.Now().UnixNano())
			}
			s.logger.Error("Dropped a batch of log entries", "entries", len(entries), "storage_full", isStorageFull(err), "error", err)
			return
		}
	}
	s.fullAt.Store(0)
	s.written.Add(stored)
	s.dupes.Add(int64(len(entries)) - stored)
	s.hub.publish(entries)
}

// insert writes the entries in one transaction and returns how many rows were really
// stored (rows ignored for a repeated client id are not counted).
func (s *Store) insert(entries []Entry) (int64, error) {
	var stored int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		stored = 0
		for start := 0; start < len(entries); start += insertChunk {
			end := min(start+insertChunk, len(entries))
			sql := "INSERT OR IGNORE INTO log_entries (id, app_id, client_id, ts, ingested_at, severity, severity_number, event, body, body_json, attributes, resource, trace_id, span_id, request_id, size, severity_inferred) VALUES "
			args := make([]any, 0, (end-start)*17)
			for i, e := range entries[start:end] {
				if i > 0 {
					sql += ","
				}
				sql += "(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"
				args = append(args, e.ID, e.AppID, e.ClientID, e.Timestamp.UnixNano(), e.IngestedAt.UnixNano(), e.Severity, e.SeverityNumber, e.Event, e.Body, e.BodyJSON, encodeJSON(e.Attributes), encodeJSON(e.Resource), e.TraceID, e.SpanID, e.RequestID, e.Size, e.SeverityInferred)
			}
			res := tx.Exec(sql, args...)
			if res.Error != nil {
				return res.Error
			}
			stored += res.RowsAffected
		}
		return nil
	})
	return stored, err
}

var entrySequence atomic.Uint32

// NewEntryID returns a time-ordered id that is unique even for many entries created in
// the same nanosecond: 16 hex digits of the clock, an 8-digit process-wide counter (so
// two calls never collide in one process) and 64 random bits (so processes never collide).
func NewEntryID(now time.Time) string {
	var r [8]byte
	_, _ = rand.Read(r[:])
	return fmt.Sprintf("log_%016x%08x%s", now.UnixNano(), entrySequence.Add(1), hex.EncodeToString(r[:]))
}

func isStorageFull(err error) bool {
	return err != nil && strings.Contains(err.Error(), "database or disk is full")
}

// applySchema creates the tables and adds columns introduced after the first release.
func applySchema(database *gorm.DB) error {
	if err := database.Exec(schema).Error; err != nil {
		return fmt.Errorf("apply logs schema: %w", err)
	}
	var cols []struct{ Name string }
	if err := database.Raw("SELECT name FROM pragma_table_info('log_entries')").Scan(&cols).Error; err != nil {
		return fmt.Errorf("read logs schema: %w", err)
	}
	for _, c := range cols {
		if c.Name == "severity_inferred" {
			return nil
		}
	}
	if err := database.Exec("ALTER TABLE log_entries ADD COLUMN severity_inferred INTEGER NOT NULL DEFAULT 0").Error; err != nil {
		return fmt.Errorf("add severity_inferred: %w", err)
	}
	return nil
}
