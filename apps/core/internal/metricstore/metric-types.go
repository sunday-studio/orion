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
	// DefaultMaxSeries bounds the series one query returns.
	DefaultMaxSeries = 100
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
	// Value is the single number of an app-reported (OTLP) gauge or sum. For a sum it is the increase
	// in the interval: the writer turns a cumulative reading into the difference to the last one.
	Value *float64
	// Histogram is the distribution of an app-reported histogram or summary. Value is nil then.
	Histogram *Histogram
}

// Temporality tells the writer how to read the number or distribution of a sum, histogram or summary
// sample, and carries the timestamps the writer needs to convert and de-duplicate it. Gauges and OS
// samples have none.
type Temporality struct {
	// Cumulative values grow since Start; the writer stores their difference to the previous reading.
	// Delta values already are the change in one interval.
	Cumulative bool
	// Start is when the cumulative value began counting (zero when the exporter did not say). A new
	// Start means the exporter restarted.
	Start time.Time
	// At is the full-resolution time of the reading, newer readings of a series win over older ones.
	At time.Time
}

// Sample is one series' measurements in a Batch.
type Sample struct {
	Key    string // stable identity inside its owner: a Node resource key, or metric name plus labels
	Kind   string // process, unit, container, ... for OS series; gauge, sum, histogram or summary for OTLP
	Name   string // display name
	Metric string // OTLP metric name ("" for OS series)
	Unit   string
	Labels string // canonical JSON of the OTLP attributes ("" for OS series)
	Values Values
	// Temporality is set for sums, histograms and summaries; the writer then converts cumulative
	// readings to per-interval changes and drops readings that are not newer than the last one.
	Temporality *Temporality
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
	// MaxSeries bounds how many series come back; 0 means DefaultMaxSeries. QueryResult.Truncated says when more matched.
	MaxSeries int
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
	// Value is the average of a gauge in the bucket, and the total increase of a sum (divide by the step
	// for a rate). ValueMax is the peak of a gauge, and the largest one-minute increase of a sum.
	Value    *float64
	ValueMax *float64
	// The fields below are set for histogram and summary series only. Count and Sum cover the whole
	// bucket; Avg is Sum/Count. A percentile is absent when it cannot be computed (a summary that did not
	// report that quantile, a histogram without buckets, no observations).
	Count     *int64
	Sum       *float64
	Avg       *float64
	Min       *float64
	Max       *float64
	P50       *float64
	P95       *float64
	P99       *float64
	Quantiles []Quantile // a summary's own quantiles, as sent
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
	// Truncated is true when more series matched than MaxSeries.
	Truncated   bool
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
	RawRows    int64 // one-minute rows (samples and histogram samples)
	HourlyRows int64 // hourly rows (samples and histogram rollups)
	Series     int64
}

// Stats are cumulative since process start.
type Stats struct {
	Written       int64 // sample rows written
	DroppedFull   int64 // samples refused because the write queue was full
	DroppedCap    int64 // samples dropped by a series cap
	DroppedStale  int64 // readings dropped because they were not newer than the series' last one (duplicates and out of order)
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
