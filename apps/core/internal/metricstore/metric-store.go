package metricstore

import (
	"errors"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"sync"
	"sync/atomic"

	"gorm.io/gorm"
)

// ErrQueueFull means the in-process write queue is at capacity. Callers drop the batch: a missing
// sample is filled by the next report, and the request path must never wait for metrics.db.
var ErrQueueFull = errors.New("metric write queue is full")

const (
	metricsDatabaseFileName = "metrics.db"
	queueDepth              = 1024
)

const schema = `
PRAGMA auto_vacuum = INCREMENTAL;
CREATE TABLE IF NOT EXISTS series (
  id INTEGER PRIMARY KEY,
  source TEXT NOT NULL,
  node_id TEXT NOT NULL DEFAULT '',
  app_id TEXT NOT NULL DEFAULT '',
  key TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL DEFAULT '',
  metric TEXT NOT NULL DEFAULT '',
  unit TEXT NOT NULL DEFAULT '',
  labels TEXT NOT NULL DEFAULT '',
  first_seen INTEGER NOT NULL,
  last_seen INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_series_ref ON series(source, node_id, app_id, key);
CREATE INDEX IF NOT EXISTS idx_series_last_seen ON series(last_seen);
CREATE TABLE IF NOT EXISTS samples (
  series_id INTEGER NOT NULL,
  ts INTEGER NOT NULL,
  cpu_percent REAL,
  memory_bytes INTEGER,
  io_read_bps REAL,
  io_write_bps REAL,
  net_rx_bps REAL,
  net_tx_bps REAL,
  pids INTEGER,
  storage_bytes INTEGER,
  value REAL,
  PRIMARY KEY (series_id, ts)
) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS samples_hourly (
  series_id INTEGER NOT NULL,
  hour INTEGER NOT NULL,
  n INTEGER NOT NULL,
  cpu_avg REAL, cpu_max REAL,
  mem_avg REAL, mem_max REAL,
  io_read_avg REAL, io_write_avg REAL,
  net_rx_avg REAL, net_tx_avg REAL,
  pids_max INTEGER, storage_max INTEGER,
  value_avg REAL, value_max REAL,
  PRIMARY KEY (series_id, hour)
) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS meta (k TEXT PRIMARY KEY, v INTEGER NOT NULL) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS dirty_hours (
  series_id INTEGER NOT NULL,
  hour INTEGER NOT NULL,
  PRIMARY KEY (series_id, hour)
) WITHOUT ROWID;
`

type job struct {
	batch Batch
	flush chan struct{} // set on a barrier job (tests, shutdown)
}

// Store is the SQLite MetricStore. One goroutine writes; readers use the pool.
type Store struct {
	db     *gorm.DB
	logger *utils.Logger

	jobs   chan job
	done   chan struct{}
	sendMu sync.RWMutex
	closed atomic.Bool

	written       atomic.Int64
	droppedFull   atomic.Int64
	droppedCap    atomic.Int64
	batchesFailed atomic.Int64

	cache map[seriesKey]cachedSeries // touched only by the writer goroutine
	// rolledThrough is the hour (unix seconds) the last Rollup reached. A sample older than that is
	// late: its hour is marked dirty so the next Rollup summarises it again.
	rolledThrough atomic.Int64
}

var _ MetricStore = (*Store)(nil)

// Open opens metrics.db in dataDir and starts the writer.
func Open(dataDir string, logger *utils.Logger) (*Store, error) {
	database, err := db.OpenFile(dataDir, metricsDatabaseFileName, "warn")
	if err != nil {
		return nil, err
	}
	return OpenWith(database, logger)
}

// OpenWith starts a Store on an existing handle (tests).
func OpenWith(database *gorm.DB, logger *utils.Logger) (*Store, error) {
	if err := database.Exec(schema).Error; err != nil {
		return nil, fmt.Errorf("apply metrics schema: %w", err)
	}
	s := &Store{db: database, logger: logger, jobs: make(chan job, queueDepth), done: make(chan struct{}), cache: map[seriesKey]cachedSeries{}}
	if through, ok, err := s.readRolledThrough(); err == nil && ok {
		s.rolledThrough.Store(through)
	}
	go s.writer()
	return s, nil
}

// DB exposes the handle for diagnostics and tests.
func (s *Store) DB() *gorm.DB { return s.db }

// Enqueue implements MetricStore.
func (s *Store) Enqueue(b Batch) error {
	if len(b.Samples) == 0 {
		return nil
	}
	s.sendMu.RLock()
	defer s.sendMu.RUnlock()
	if s.closed.Load() {
		return ErrQueueFull
	}
	select {
	case s.jobs <- job{batch: b}:
		return nil
	default:
		s.droppedFull.Add(int64(len(b.Samples)))
		return ErrQueueFull
	}
}

// Flush blocks until every batch accepted before the call is written.
func (s *Store) Flush() {
	s.sendMu.RLock()
	defer s.sendMu.RUnlock()
	if s.closed.Load() {
		return
	}
	barrier := make(chan struct{})
	s.jobs <- job{flush: barrier}
	<-barrier
}

// Close drains the queue and stops the writer.
func (s *Store) Close() {
	s.sendMu.Lock()
	if s.closed.Swap(true) {
		s.sendMu.Unlock()
		return
	}
	close(s.jobs)
	s.sendMu.Unlock()
	<-s.done
}

func (s *Store) writer() {
	defer close(s.done)
	for j := range s.jobs {
		if j.flush != nil {
			close(j.flush)
			continue
		}
		if err := s.writeBatch(j.batch); err != nil {
			s.batchesFailed.Add(1)
			s.logger.Warn("Could not write metric samples", "error", err, "source", j.batch.Source)
		}
	}
}

// Stats implements MetricStore.
func (s *Store) Stats() Stats {
	st := Stats{Written: s.written.Load(), DroppedFull: s.droppedFull.Load(), DroppedCap: s.droppedCap.Load(),
		BatchesFailed: s.batchesFailed.Load(), QueueDepth: len(s.jobs)}
	var pages, size int64
	s.db.Raw("PRAGMA page_count").Scan(&pages)
	s.db.Raw("PRAGMA page_size").Scan(&size)
	st.DatabaseBytes = pages * size
	return st
}
