package tracestore

import (
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
var ErrQueueFull = errors.New("trace ingest queue is full")

// ErrStorageFull means the last write failed because the disk or database is full. Enqueue
// refuses new spans for storageFullBackoff (answer 503 + Retry-After) instead of accepting
// data it cannot persist, then lets one batch probe whether space has returned.
var ErrStorageFull = errors.New("trace storage is full")

var storageFullBackoff = 5 * time.Second // var so tests can shorten it

const (
	// DefaultQueueMax is how many spans may wait for the writer.
	DefaultQueueMax        = 50_000
	writerBatchSpans       = 1000
	writerBatchWait        = 50 * time.Millisecond
	insertChunk            = 400
	tracesDatabaseFileName = "traces.db"
)

const schema = `
CREATE TABLE IF NOT EXISTS spans (
  app_id TEXT NOT NULL,
  trace_id TEXT NOT NULL,
  span_id TEXT NOT NULL,
  parent_span_id TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL,
  kind INTEGER NOT NULL DEFAULT 0,
  start_ns INTEGER NOT NULL,
  duration_ns INTEGER NOT NULL DEFAULT 0,
  status_code INTEGER NOT NULL DEFAULT 0,
  status_message TEXT NOT NULL DEFAULT '',
  service_name TEXT NOT NULL DEFAULT '',
  attributes TEXT NOT NULL DEFAULT '',
  resource TEXT NOT NULL DEFAULT '',
  events TEXT NOT NULL DEFAULT '',
  links TEXT NOT NULL DEFAULT '',
  ingested_at INTEGER NOT NULL,
  size INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_spans_trace_app_span ON spans(trace_id, app_id, span_id);
CREATE INDEX IF NOT EXISTS idx_spans_app_start ON spans(app_id, start_ns);
CREATE TABLE IF NOT EXISTS traces (
  app_id TEXT NOT NULL,
  trace_id TEXT NOT NULL,
  root_name TEXT NOT NULL DEFAULT '',
  service_name TEXT NOT NULL DEFAULT '',
  start_ns INTEGER NOT NULL,
  duration_ns INTEGER NOT NULL DEFAULT 0,
  span_count INTEGER NOT NULL DEFAULT 0,
  error_count INTEGER NOT NULL DEFAULT 0,
  has_root INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL,
  PRIMARY KEY (app_id, trace_id)
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS idx_traces_app_start ON traces(app_id, start_ns DESC, trace_id DESC);
CREATE INDEX IF NOT EXISTS idx_traces_start ON traces(start_ns DESC, trace_id DESC, app_id DESC);
CREATE INDEX IF NOT EXISTS idx_traces_trace ON traces(trace_id);
CREATE INDEX IF NOT EXISTS idx_traces_app_errors ON traces(app_id, start_ns DESC) WHERE error_count > 0;
CREATE INDEX IF NOT EXISTS idx_traces_app_duration ON traces(app_id, duration_ns DESC);
`

// Stats are cumulative since process start.
type Stats struct {
	Queued  int64 `json:"queued"`
	Written int64 `json:"written"`
	Dropped int64 `json:"dropped"`
	// Duplicates are spans ignored because the same span id was already stored (exporter retries).
	Duplicates int64 `json:"duplicates"`
	// StorageFull is true while the last write failed with a disk-full error.
	StorageFull bool `json:"storage_full"`
}

// Store owns traces.db, the bounded queue in front of it and the single writer.
type Store struct {
	db       *gorm.DB
	logger   *utils.Logger
	queueMax int64

	batches chan []Span
	queued  atomic.Int64
	written atomic.Int64
	dropped atomic.Int64
	dupes   atomic.Int64
	// fullAt is the unix-nano time of the last disk-full write failure (0 = healthy).
	fullAt atomic.Int64

	// sendMu orders Enqueue's send against Close: senders hold the read lock, Close the write lock.
	sendMu sync.RWMutex

	closeOnce sync.Once
	closed    atomic.Bool
	done      chan struct{}
}

// Open opens (creating if needed) traces.db in dataDir, applies the schema and starts the writer.
func Open(dataDir string, logger *utils.Logger, queueMax int) (*Store, error) {
	database, err := db.OpenFile(dataDir, tracesDatabaseFileName, "warn")
	if err != nil {
		return nil, err
	}
	return OpenWith(database, logger, queueMax)
}

// OpenWith starts a Store on an existing handle (tests).
func OpenWith(database *gorm.DB, logger *utils.Logger, queueMax int) (*Store, error) {
	if err := database.Exec(schema).Error; err != nil {
		return nil, fmt.Errorf("apply traces schema: %w", err)
	}
	if queueMax <= 0 {
		queueMax = DefaultQueueMax
	}
	s := &Store{db: database, logger: logger, queueMax: int64(queueMax), batches: make(chan []Span, 4096), done: make(chan struct{})}
	go s.writer()
	return s, nil
}

// DB exposes the traces handle for diagnostics and tests.
func (s *Store) DB() *gorm.DB { return s.db }

// Stats reports the writer counters.
func (s *Store) Stats() Stats {
	return Stats{Queued: s.queued.Load(), Written: s.written.Load(), Dropped: s.dropped.Load(), Duplicates: s.dupes.Load(), StorageFull: s.fullAt.Load() != 0}
}

// QueueDepth returns the queued and maximum span counts of the write queue.
func (s *Store) QueueDepth() (queued, max int64) { return s.queued.Load(), s.queueMax }

// Enqueue accepts a batch for writing without blocking. It fails with ErrQueueFull when the
// queue would exceed its span budget, so a slow disk turns into 429s instead of unbounded memory.
func (s *Store) Enqueue(spans []Span) error {
	if len(spans) == 0 {
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
	n := int64(len(spans))
	if s.queued.Add(n) > s.queueMax {
		s.queued.Add(-n)
		return ErrQueueFull
	}
	select {
	case s.batches <- spans:
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

// Close stops accepting spans, drains the queue and waits for the writer.
func (s *Store) Close() {
	s.closeOnce.Do(func() {
		s.sendMu.Lock() // waits for in-flight Enqueue calls, then blocks new ones
		if !s.closed.Swap(true) {
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
		for len(pending) < writerBatchSpans {
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

func (s *Store) flushBatch(spans []Span) {
	defer s.queued.Add(-int64(len(spans)))
	stored, err := s.insert(spans)
	if err != nil {
		// One retry covers a transient lock; after that the batch is counted as dropped and logged.
		time.Sleep(100 * time.Millisecond)
		if stored, err = s.insert(spans); err != nil {
			s.dropped.Add(int64(len(spans)))
			if isStorageFull(err) {
				s.fullAt.Store(time.Now().UnixNano())
			}
			s.logger.Error("Dropped a batch of spans", "spans", len(spans), "storage_full", isStorageFull(err), "error", err)
			return
		}
	}
	s.fullAt.Store(0)
	s.written.Add(stored)
	s.dupes.Add(int64(len(spans)) - stored)
}

func isStorageFull(err error) bool {
	return err != nil && strings.Contains(err.Error(), "database or disk is full")
}
