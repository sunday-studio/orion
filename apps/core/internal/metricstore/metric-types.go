// Package metricstore keeps metric samples (OS-measured app resources and, later, app-reported
// OTLP metrics) in their own SQLite file, metrics.db, so their volume never touches orion.db or its
// migration backups. Everything goes through the MetricStore interface.
package metricstore

import "time"

// Sources of a series: measured by a Node on the host, or reported by an app over OTLP.
const (
	SourceOS   = "os"
	SourceOTLP = "otlp"
)

const (
	// DefaultSeriesCapPerNode matches the cap the Node already applies (200 series per host).
	DefaultSeriesCapPerNode = 200
	// DefaultSeriesCapPerApp is deliberately low: apps can emit unbounded label sets.
	DefaultSeriesCapPerApp = 100
	// MaxAppSeries bounds how many OTLP series a query returns for a set of apps.
	MaxAppSeries = 50
	// ActiveSeriesWindow is how recently a series must have reported to count against its cap, so a
	// container that was redeployed under a new key does not hold a slot for months.
	ActiveSeriesWindow = 24 * time.Hour
)

// Values are the measurements of one series at one minute. A nil field was not measured; it is
// never stored as zero.
type Values struct {
	CPUPercent   *float64
	MemoryBytes  *int64
	IOReadBps    *float64
	IOWriteBps   *float64
	NetRxBps     *float64
	NetTxBps     *float64
	Pids         *int64
	StorageBytes *int64
	// Value is the single number of an app-reported (OTLP) gauge or sum.
	Value *float64
}

// Sample is one series' measurements in a Batch.
type Sample struct {
	Key    string // stable identity inside its owner: a Node resource key, or metric name plus labels
	Kind   string // process, unit, container, ... for OS series; gauge or sum for OTLP
	Name   string // display name
	Metric string // OTLP metric name ("" for OS series)
	Unit   string
	Labels string // canonical JSON of the OTLP attributes ("" for OS series)
	Values Values
}

// Batch is what one Node report or one OTLP request contributes. Samples are stored at minute
// resolution: a second batch inside the same minute overwrites the first.
type Batch struct {
	Source string
	NodeID string // owner of OS series
	AppID  string // owner of OTLP series
	At     time.Time
	// SeriesCap bounds the active series of this owner; 0 uses the default for the source. A sample
	// that would exceed it is dropped and counted.
	SeriesCap int
	Samples   []Sample
}

// SeriesRef identifies one series.
type SeriesRef struct {
	Source string
	NodeID string
	AppID  string
	Key    string
}

// Query asks for the history of some series between From and To.
type Query struct {
	Series []SeriesRef
	// AppIDs adds the most recently seen OTLP series of these apps, at most MaxAppSeries in all.
	AppIDs []string
	From   time.Time
	To     time.Time
	// MaxPoints is about how many points a series returns (at most one more, from bucket alignment); the step grows to fit. 0 means 500, the maximum is 2000.
	MaxPoints int
}

// Point is one bucket of a series. Averages are over the bucket; Max fields are its peak.
type Point struct {
	T              time.Time
	CPUPercent     *float64
	CPUPercentMax  *float64
	MemoryBytes    *float64
	MemoryBytesMax *float64
	IOReadBps      *float64
	IOWriteBps     *float64
	NetRxBps       *float64
	NetTxBps       *float64
	PidsMax        *int64
	StorageMax     *int64
	Value          *float64
	ValueMax       *float64
}

// SeriesResult is one series with its points, oldest first.
type SeriesResult struct {
	Ref    SeriesRef
	Kind   string
	Name   string
	Metric string
	Unit   string
	Labels string
	Points []Point
}

// QueryResult carries the resolution that was used: "raw" (one-minute rows) or "hourly" (rollups).
type QueryResult struct {
	Resolution  string
	StepSeconds int
	Series      []SeriesResult
}

// Retention says how long each resolution is kept.
type Retention struct {
	Raw    time.Duration
	Rollup time.Duration
}

// PruneResult counts what a prune removed.
type PruneResult struct {
	RawRows    int64
	HourlyRows int64
	Series     int64
}

// Stats are cumulative since process start.
type Stats struct {
	Written       int64 // sample rows written
	DroppedFull   int64 // samples refused because the write queue was full
	DroppedCap    int64 // samples dropped by a series cap
	BatchesFailed int64
	QueueDepth    int
	DatabaseBytes int64
}

// MetricStore is the storage behind resource history and app metrics. The SQLite implementation is
// Store; the interface lets the engine change without touching callers.
type MetricStore interface {
	// Enqueue accepts a batch for writing without blocking. It fails with ErrQueueFull rather than
	// slowing the caller.
	Enqueue(b Batch) error
	// AllowSeries says which series keys of an owner may be written under cap (0 = the source's default).
	AllowSeries(source, nodeID, appID string, keys []string, cap int) (map[string]bool, error)
	Query(q Query) (QueryResult, error)
	Rollup(now time.Time) (int, error)
	Prune(r Retention, now time.Time) (PruneResult, error)
	Stats() Stats
	Close()
}
