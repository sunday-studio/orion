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
	Value          *float64  `json:"value,omitempty"`
	ValueMax       *float64  `json:"value_max,omitempty"`
}

// ServiceMetricSeries is the history of one linked resource or one app-reported metric.
type ServiceMetricSeries struct {
	Source string               `json:"source" enums:"os,otlp"`
	NodeID string               `json:"node_id,omitempty"`
	AppID  string               `json:"app_id,omitempty"`
	Key    string               `json:"key"`
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
	Resolution  string                `json:"resolution" enums:"raw,hourly"`
	StepSeconds int                   `json:"step_seconds"`
	Series      []ServiceMetricSeries `json:"series"`
}
