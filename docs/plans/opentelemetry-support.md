# Plan: full OpenTelemetry support

Status: in progress. Decided 2026-11-20: cover metrics, traces and gRPC, and build the Console trace views in the first pass.

## Where we are

| Signal | Today |
|---|---|
| Logs | `POST /v1/ingest/otlp/v1/logs`, protobuf and JSON, gzip. Complete. |
| Metrics | `POST /v1/ingest/otlp/v1/metrics`, protobuf and JSON. **Gauges and sums only**; histograms, exponential histograms and summaries are rejected. |
| Traces | Nothing. Logs keep `trace_id` and `span_id`, nothing else knows about traces. |
| Transport | OTLP/HTTP only. No gRPC receiver. |

Everything authenticates with an app ingest key (`Authorization: Bearer`) and lands under an app, which a service links to.

## Principles

- Same shape as what exists. Metrics stay in `metrics.db` behind `MetricStore`; traces get `traces.db` with a bounded write queue like `logstore`, so a slow disk never delays an OTLP request and a full queue answers 429 with `Retry-After`.
- Every limit is explicit: series per app, spans per request, attributes per span, storage per app. Over a limit, reject or drop and say so in `partial_success`.
- One ingest key per app for all three signals. No new credentials.
- Every new route lands with OpenAPI annotations, `make generate-openapi` and `make generate-sdk`.

## Phase 1: metrics completeness

Apps send histograms for latency, so this is what most SDK users hit first.

- **Histograms (explicit buckets):** store per minute `count`, `sum`, `min`, `max`, the bucket bounds and the bucket counts, in a new `histogram_samples` table next to `samples`. A series of kind `histogram` has no single `value`.
- **Exponential histograms:** convert to the same stored shape (counts per bucket with computed bounds) so one reader handles both. Cap the stored buckets per sample.
- **Summaries:** the quantile values become stored fields; no bucket counts.
- **Temporality:** cumulative sums and histograms are turned into per-interval deltas on ingest (state per series: last value and start time, reset detection), delta ones are stored as sent. Charts then show a rate or a per-minute count, never an ever-growing counter.
- **Reading:** `GET /v1/services/{id}/metrics` returns for a histogram series `count`, `sum`, `avg`, and `p50`, `p95`, `p99` derived from the merged buckets (linear interpolation inside the bucket). Hourly rollups merge bucket counts, so percentiles stay correct over 7 and 30 days.
- **Retention:** the existing sample and rollup retention settings apply.

As built (differences from the list above; the behaviour is documented in `docs/product-metrics.md`):

- Temporality state is one row per series in `series_state` in `metrics.db`, written in the batch's transaction by the writer, so it survives restarts and a retry never counts twice; a reading that is not newer than the last accepted one is dropped (counted in the store stats, not in `partial_success`). The first cumulative reading of a series is a baseline and produces no point.
- Summary `count` and `sum` are treated as cumulative and stored as changes like a cumulative histogram; the quantiles are stored as sent.
- A non-monotonic cumulative sum is stored as a `gauge`, a non-monotonic delta sum as a `sum` that may be negative. Sum series now return the total increase per bucket in `value`.
- Existing OTLP `sum` series lose their stored history once on upgrade, because it holds cumulative values that cannot be told apart from the new per-minute changes.
- Histogram points also return `min` and `max`; a summary point returns its `quantiles` as well as `p50`, `p95` and `p99` when reported.

## Phase 2: gRPC receiver

Many SDKs default to OTLP/gRPC on port 4317.

- A gRPC server in Core registers the OTLP `LogsService`, `MetricsService` and `TraceService`, and hands requests to the same parsing and storage code as HTTP.
- Auth reads `authorization: Bearer <key>` from gRPC metadata, with the same per-app rate limits and errors mapped to gRPC status codes (`Unauthenticated`, `ResourceExhausted`, `InvalidArgument`).
- Config: `ORION_OTLP_GRPC_ADDR` (default off, example `:4317`); the Docker image exposes 4317 and the compose file publishes it only when set. `google.golang.org/grpc` is already in `go.mod` as an indirect dependency.
- Message size and connection limits match the HTTP body limits.

## Phase 3: traces backend

- **Storage:** `traces.db` with a `spans` table (trace id, span id, parent id, app id, name, kind, start, duration, status code and message, attributes, resource, events and links as JSON) and a `traces` summary table kept up to date by the writer (root span name, service, start, duration, span count, error count). Indexes for app plus start time, trace id, and errors.
- **Ingest:** `POST /v1/ingest/otlp/v1/traces` (protobuf and JSON, gzip). Same app key, same rate limiter, spans per request capped, oversized attributes truncated. Orphan spans are kept; a trace is whatever arrived.
- **Query API:**
  - `GET /v1/traces`: filter by `app_id`, `service`, `status` (ok, error), `min_duration`, `name`, `from`, `to`, `attr=key:value`; cursor pagination.
  - `GET /v1/traces/{trace_id}`: the spans of one trace, with the logs that carry its `trace_id` counted.
  - `GET /v1/traces/facets` for the filter bar.
- **Retention:** per app, reusing the app's `retention_days` and `max_storage_mb` model that logs use; a separate trace retention default in the data lifecycle settings. Pruning runs with the log retention job.
- **Services:** an app linked to a service contributes its traces; the service metrics response gains request count, error rate and latency percentiles from root spans.

## Phase 4: Console

Built in `apps/console/` with the data-table pattern (TanStack Table and Query, `nuqs` URL state, server-side filtering).

- A **Traces** page under Logs: list with search and filters, one row per trace (root name, service, duration, spans, errors).
- A **trace detail** page: span waterfall with an indented tree, per-span attributes, events and status, and the logs of the trace beside it.
- **Links:** a trace id in a log entry opens the trace; a trace's span opens the matching logs; a service page shows recent traces and its error rate and latency.
- Histogram metric series render as percentile lines on the service page.

## Phase 5: docs and contracts

- `docs/product-traces.md`, and updates to `product-logs.md`, `product-metrics.md`, `how-it-works.md` and the website install and connect pages: one section "Connect OpenTelemetry" that covers the three signals, HTTP and gRPC.
- `docs/node-core-contract.md` only if a Node field changes (none planned).
- Core environment reference for the new settings.
- A tested compatibility matrix: official SDK and Collector exporters against Orion, in CI where practical.

## Order and dependencies

1. Phase 1 and phase 3 are independent of each other (`metricstore` against a new traces package). Phase 2 needs the handlers of 1 and 3 to exist, so it lands after them. Phase 4 needs the phase 3 API.
2. Phase 1 first: smallest, and unblocks real latency data.

## Out of scope

- Profiles (the fourth OTLP signal) and OpenTelemetry Collector-side features such as tail sampling.
- Sending data out of Orion over OTLP.
- Per-span alerting. A trace-based alert rule is a later step once traces are stored.
