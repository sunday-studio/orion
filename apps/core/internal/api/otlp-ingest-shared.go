package api

import (
	"errors"
	"orion/core/internal/logstore"
	"orion/core/internal/metricstore"
	"orion/core/internal/tracestore"
	"strconv"
	"time"

	collogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collmetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

// The functions here are the one ingest path of OTLP/HTTP and OTLP/gRPC. A transport authenticates the
// caller, decodes the request message, calls one of exportLogs, exportMetrics or exportTraces and maps the
// result or the ingestFailure onto its own wire (status codes and headers, or gRPC codes and trailers).
// Limits, validation, redaction, rate limiting and the write queues live here and in the stores, so a
// request behaves the same whichever way it arrived.

// ingestFailureKind says why a whole request was refused.
type ingestFailureKind int

const (
	failUnavailable     ingestFailureKind = iota + 1 // the signal is not enabled on this Core
	failUnauthenticated                              // no valid ingest key
	failTooMany                                      // more entries, points or spans than one request may carry
	failInvalid                                      // the request cannot be read
	failRateLimited                                  // the app's ingest budget is spent
	failStorageFull                                  // the store cannot write; the data was not accepted
	failQueueFull                                    // the write queue is full; the data was not accepted
	failInternal                                     // anything else; err and message describe it
)

// ingestFailure is a refused request. retryAfter is the wait a client should observe before retrying
// (rate limit, storage full and queue full).
type ingestFailure struct {
	kind       ingestFailureKind
	retryAfter time.Duration
	message    string
	err        error
}

// retryAfterSeconds rounds a wait up to whole seconds, at least one, for Retry-After style hints.
func retryAfterSeconds(wait time.Duration) int {
	return max(int((wait+time.Second-1)/time.Second), 1)
}

func retryAfterFailure(err error) (*ingestFailure, bool) {
	var limited *logstore.RateLimitedError
	if errors.As(err, &limited) {
		return &ingestFailure{kind: failRateLimited, retryAfter: limited.RetryAfter}, true
	}
	return nil, false
}

// authenticateIngest resolves an Authorization header value to an app. A Core without product logs has no
// keys to check, so it reports the signal as unavailable, as the HTTP routes do.
func (s *Server) authenticateIngest(authorization string) (*logstore.AppContext, *ingestFailure) {
	if s.logs == nil {
		return nil, &ingestFailure{kind: failUnavailable}
	}
	app, err := s.logs.ingester.Authenticate(authorization)
	if err != nil {
		return nil, &ingestFailure{kind: failUnauthenticated}
	}
	return app, nil
}

// exportLogs ingests an OTLP logs request for an authenticated app.
func (s *Server) exportLogs(app *logstore.AppContext, request *collogs.ExportLogsServiceRequest) (logstore.Result, *ingestFailure) {
	if s.logs == nil {
		return logstore.Result{}, &ingestFailure{kind: failUnavailable}
	}
	result, err := s.logs.ingester.IngestOTLPRequest(app, request)
	if err == nil {
		return result, nil
	}
	if failure, ok := retryAfterFailure(err); ok {
		return logstore.Result{}, failure
	}
	switch {
	case errors.Is(err, logstore.ErrTooManyEntries):
		return logstore.Result{}, &ingestFailure{kind: failTooMany}
	case errors.Is(err, logstore.ErrInvalidBody):
		return logstore.Result{}, &ingestFailure{kind: failInvalid}
	case errors.Is(err, logstore.ErrStorageFull):
		return logstore.Result{}, &ingestFailure{kind: failStorageFull, retryAfter: 5 * time.Second}
	}
	return logstore.Result{}, &ingestFailure{kind: failInternal, message: "Failed to ingest logs", err: err}
}

// metricExportResult is what a metrics request reports back: the data points that were refused, and why.
type metricExportResult struct {
	Rejected int64
	Message  string
}

// exportMetrics ingests an OTLP metrics request for an authenticated app. Data points that cannot be
// stored, or that belong to series over the app's cap, are counted in the result and never fail the request.
func (s *Server) exportMetrics(app *logstore.AppContext, request *collmetrics.ExportMetricsServiceRequest) (metricExportResult, *ingestFailure) {
	if s.metrics == nil {
		return metricExportResult{}, &ingestFailure{kind: failUnavailable}
	}
	parsed, err := metricstore.ParseOTLPMetricsRequest(request, time.Now().UTC())
	switch {
	case errors.Is(err, metricstore.ErrTooManyPoints):
		return metricExportResult{}, &ingestFailure{kind: failTooMany}
	case errors.Is(err, metricstore.ErrInvalidBody):
		return metricExportResult{}, &ingestFailure{kind: failInvalid}
	case err != nil:
		return metricExportResult{}, &ingestFailure{kind: failInternal, message: "Failed to read the metrics request", err: err}
	}

	rejected, messages := parsed.Rejected, parsed.Errors
	if len(parsed.Points) > 0 {
		if failure, ok := retryAfterFailure(s.logs.ingester.Allow(app, len(parsed.Points))); ok {
			return metricExportResult{}, failure
		}
		seriesCap := app.App.MetricSeriesCap
		if seriesCap <= 0 {
			seriesCap = metricstore.DefaultSeriesCapPerApp
		}
		batches, over, err := s.planMetricBatches(app.App.ID, parsed.Points, seriesCap)
		if err != nil {
			return metricExportResult{}, &ingestFailure{kind: failInternal, message: "Failed to check the metric series limit", err: err}
		}
		if over > 0 {
			rejected += over
			messages = append(messages, "app reached its metric series limit of "+strconv.Itoa(seriesCap)+"; raise metric_series_cap on the app to accept more")
		}
		for _, b := range batches {
			if err := s.enqueueMetricBatch(b); err != nil {
				return metricExportResult{}, &ingestFailure{kind: failQueueFull, retryAfter: time.Second}
			}
		}
	}
	message := ""
	for i, m := range messages {
		if i > 0 {
			message += "; "
		}
		message += m
	}
	return metricExportResult{Rejected: rejected, Message: message}, nil
}

// traceExportResult is what a traces request reports back: how many spans were refused, and why.
type traceExportResult struct {
	Rejected int
	Errors   []tracestore.SpanError
}

// exportTraces ingests an OTLP traces request for an authenticated app. A span that breaks a rule is
// refused on its own and reported in the result; the rest of the request is stored.
func (s *Server) exportTraces(app *logstore.AppContext, request *coltrace.ExportTraceServiceRequest) (traceExportResult, *ingestFailure) {
	if s.logs == nil || s.traces == nil {
		return traceExportResult{}, &ingestFailure{kind: failUnavailable}
	}
	parsed, err := tracestore.ParseOTLPRequest(request, time.Now().UTC())
	switch {
	case errors.Is(err, tracestore.ErrTooManySpans):
		return traceExportResult{}, &ingestFailure{kind: failTooMany}
	case errors.Is(err, tracestore.ErrInvalidBody):
		return traceExportResult{}, &ingestFailure{kind: failInvalid}
	case err != nil:
		return traceExportResult{}, &ingestFailure{kind: failInternal, message: "Failed to read the trace request", err: err}
	}

	if len(parsed.Spans) > 0 {
		if failure, ok := retryAfterFailure(s.logs.ingester.Allow(app, len(parsed.Spans))); ok {
			return traceExportResult{}, failure
		}
		for i := range parsed.Spans {
			parsed.Spans[i].AppID = app.App.ID
		}
		tracestore.Redact(parsed.Spans, app.Redactor)
		switch err := s.traces.Enqueue(parsed.Spans); {
		case errors.Is(err, tracestore.ErrStorageFull):
			return traceExportResult{}, &ingestFailure{kind: failStorageFull, retryAfter: 5 * time.Second}
		case err != nil:
			return traceExportResult{}, &ingestFailure{kind: failQueueFull, retryAfter: time.Second}
		}
	}
	return traceExportResult{Rejected: len(parsed.Errors), Errors: parsed.Errors}, nil
}
