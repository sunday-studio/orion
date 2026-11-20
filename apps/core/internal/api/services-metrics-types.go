package api

import "time"

// ServiceMetricPoint is one bucket of a metric series. A field that was not measured is absent.
// Rates are per second; *_max fields are the peak inside the bucket, the others its average.
type ServiceMetricPoint struct {
	T              time.Time `json:"t"`
	CPUPercent     *float64  `json:"cpu_percent,omitempty"`
	CPUPercentMax  *float64  `json:"cpu_percent_max,omitempty"`
	MemoryBytes    *float64  `json:"memory_bytes,omitempty"`
	MemoryBytesMax *float64  `json:"memory_bytes_max,omitempty"`
	IOReadBps      *float64  `json:"io_read_bytes_per_sec,omitempty"`
	IOWriteBps     *float64  `json:"io_write_bytes_per_sec,omitempty"`
	NetRxBps       *float64  `json:"net_rx_bytes_per_sec,omitempty"`
	NetTxBps       *float64  `json:"net_tx_bytes_per_sec,omitempty"`
	PidsMax        *int64    `json:"pids_max,omitempty"`
	StorageMax     *int64    `json:"storage_bytes_max,omitempty"`
	// Gauge series: the average in the bucket. Sum series: the total increase in the bucket (divide by step_seconds for a rate).
	Value *float64 `json:"value,omitempty"`
	// Gauge series: the peak in the bucket. Sum series: the largest one-minute increase.
	ValueMax *float64 `json:"value_max,omitempty"`
	// Histogram and summary series: observations in the bucket.
	Count *int64 `json:"count,omitempty"`
	// Histogram and summary series: total of the observed values in the bucket (absent when the exporter sent no sum).
	Sum *float64 `json:"sum,omitempty"`
	// Histogram and summary series: sum divided by count (absent without observations or a sum).
	Avg *float64 `json:"avg,omitempty"`
	// Histogram series: smallest observed value, when the exporter sent it (delta temporality).
	Min *float64 `json:"min,omitempty"`
	// Histogram series: largest observed value, when the exporter sent it (delta temporality).
	Max *float64 `json:"max,omitempty"`
	// Histogram series: median from the merged bucket counts of the bucket (linear interpolation inside a bucket). Summary series: the reported 0.5 quantile. Absent when it cannot be computed.
	P50 *float64 `json:"p50,omitempty"`
	// As p50, for the 95th percentile.
	P95 *float64 `json:"p95,omitempty"`
	// As p50, for the 99th percentile.
	P99 *float64 `json:"p99,omitempty"`
	// Summary series: the quantiles the exporter reported, averaged over the bucket weighted by observations.
	Quantiles []ServiceMetricQuantile `json:"quantiles,omitempty"`
}

// ServiceMetricQuantile is one quantile (0 to 1) of a summary series.
type ServiceMetricQuantile struct {
	Quantile float64 `json:"quantile"`
	Value    float64 `json:"value"`
}

// ServiceMetricSeries is the history of one linked resource or one app-reported metric.
type ServiceMetricSeries struct {
	Source string `json:"source" enums:"os,otlp"`
	NodeID string `json:"node_id,omitempty"`
	AppID  string `json:"app_id,omitempty"`
	Key    string `json:"key"`
	// Kind is process, unit, container and so on for measured resources, and gauge, sum, histogram or
	// summary for app-reported metrics. A sum holds increases per interval (Orion turns cumulative sums
	// into them), a gauge a level.
	Kind   string               `json:"kind,omitempty"`
	Name   string               `json:"name"`
	Metric string               `json:"metric,omitempty"`
	Unit   string               `json:"unit,omitempty"`
	Labels string               `json:"labels,omitempty"`
	Points []ServiceMetricPoint `json:"points"`
}

// ServiceMetricsResponse is the metric history of a service's linked resources and apps.
type ServiceMetricsResponse struct {
	ServiceID string    `json:"service_id"`
	From      time.Time `json:"from"`
	To        time.Time `json:"to"`
	// Resolution is "raw" (one-minute rows, ranges up to 48 hours) or "hourly" (rollups).
	Resolution string `json:"resolution" enums:"raw,hourly"`
	// Truncated is true when the service has more series than the response carries: 100 in all, of which at most 50 are app-reported (the most recently seen).
	Truncated   bool                  `json:"truncated"`
	StepSeconds int                   `json:"step_seconds"`
	Series      []ServiceMetricSeries `json:"series"`
}
