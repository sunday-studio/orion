# Product Traces

How an app sends traces to Orion and how Core stores and serves them. Plan: [plans/opentelemetry-support.md](plans/opentelemetry-support.md), phase 3. Logs are in [product-logs.md](product-logs.md) and metrics in [product-metrics.md](product-metrics.md).

Status: **backend live**: OTLP/HTTP ingest, storage, retention and the read API. The Console trace views come next; until then use the API.

## 1. Connect an app

Point the OTLP/HTTP traces exporter at Orion with the app's ingest key (the same key as logs and metrics, from **Logs > Apps**):

```sh
export OTEL_EXPORTER_OTLP_TRACES_ENDPOINT="$ORION_URL/v1/ingest/otlp/v1/traces"
export OTEL_EXPORTER_OTLP_TRACES_HEADERS="Authorization=Bearer $ORION_INGEST_KEY"
export OTEL_EXPORTER_OTLP_TRACES_PROTOCOL="http/json"   # http/protobuf also works
export OTEL_SERVICE_NAME="checkout-api"
```

Link the app to a [service](plans/service-correlation-plan.md) and the service's traces, error rate and latency come with it. An OpenTelemetry Collector works the same way: an `otlphttp` exporter with `traces_endpoint` and an `Authorization` header.

## 2. What is stored

Core stores every span as sent, in `traces.db` (next to `orion.db` and `logs.db` in `ORION_DATA_DIR`): ids, parent, name, kind, start and duration in nanoseconds, status, attributes, the resource, events and links. Scope name and version are kept as the `otel.scope.name` and `otel.scope.version` attributes. The resource's `service.name` is also kept in its own column for filtering.

Core also keeps one **trace summary** per app and trace id, maintained by the writer: root span name, service, start, duration, span count, error count and whether the root has arrived.

- A trace is whatever arrived. Spans of one trace may come in any order and in separate requests (a child is usually exported before its parent). Orphan spans, whose parent never arrives, are kept.
- The summary is rebuilt from the stored spans every time a batch touches the trace, so it is the same whatever the arrival order. While no root span has arrived, the summary is named after the earliest span, its duration is the extent of the spans, and `has_root` is false; the root, when it arrives, takes over.
- The root is the earliest span with no parent. The trace duration is the root span's duration.
- Sending the same span twice (an exporter retry) stores it once and counts it once.
- A trace id seen by two apps (both sides of a call are instrumented, each with its own key) is two summaries, one per app, each with that app's spans. `GET /v1/traces/{trace_id}` returns the spans of all apps.

## 3. Limits, errors and retries

| Limit | Value |
|---|---|
| Spans per request | 10,000 (413 beyond) |
| Request body | 5 MiB, also after gzip decompression (413 beyond) |
| One span | 256 KiB, measured as stored (every column, the resource included) and not only as sent; larger spans are refused |
| Attributes per span | 128, the first ones in the order sent; the rest are dropped |
| Resource attributes | 128 and 8 KiB of encoded JSON on each span; service, host, `k8s.*` and `cloud.*` keys are kept first, then the rest in key order |
| Events and links per span | 128 each, 32 attributes per event or link |
| Text values | 4,096 bytes, cut on a character boundary |
| Attribute nesting | 8 levels (arrays and maps) |
| Start time | at most 1 hour in the future |

Cutting is never a rejection: the span records what it lost in the attributes `orion.dropped_attributes`, `orion.dropped_events`, `orion.dropped_links`, `orion.dropped_resource_attributes` and `orion.truncated_values`. The resource is copied onto every span of a request, which is why it is capped: a 500 KB resource sent with 200 small spans would otherwise be stored 200 times. The size that `max_storage_mb` and the 256 KiB limit use is the stored bytes of a span (its text columns plus a fixed 128 bytes for the app id and numbers), not the size on the wire. Spans stored by an earlier Core keep the smaller size they were given then.

A request is accepted with 200 and an OTLP `ExportTraceServiceResponse` in the encoding the client used. Spans that break a rule are counted in `partial_success.rejected_spans` with up to five reasons in `error_message`; the rest of the request is stored. A rejected span has one of these reasons:

| Code | Meaning |
|---|---|
| `invalid_trace_id` | not 16 bytes, or all zero |
| `invalid_span_id` | not 8 bytes, or all zero |
| `invalid_parent_span_id` | present but not 8 bytes (an empty or all-zero parent means no parent) |
| `invalid_timestamp` | no start or end time, an end before the start, or a start over an hour ahead |
| `span_too_large` | over 256 KiB as stored |
| `attributes_too_deep` | a value nested over 8 levels |

An empty span name is stored as `(unnamed)`.

Request-level answers are Orion JSON envelopes, as for logs and metrics:

| Status | Meaning | What to do |
|---|---|---|
| `400` | not a valid `ExportTraceServiceRequest`, or bad gzip | fix the sender |
| `401` | missing, unknown or revoked ingest key | check the key |
| `413` | too many spans or too large a body | send smaller batches |
| `415` | not `application/x-protobuf` or `application/json` | change the exporter protocol |
| `429` | the app's ingest rate limit, or the write queue is full | retry after `Retry-After` |
| `503` | traces are off on this Core, or `traces.db` is out of space | retry after `Retry-After`; free disk space |

The rate limit is the app's: each span counts as one entry against the same budget as log lines and metric data points. Writing is asynchronous through a bounded queue (50,000 spans) and a single writer, so a slow disk answers 429 instead of delaying requests. The app's redaction rules (default secret keys such as `password`, `token` and `authorization`, plus its own keys and regexes) apply to span, event, link and resource attributes and to the status message. A key containing `token` is redacted, which includes names such as `gen_ai.usage.input_tokens`; turn the default keys off on the app if you need those.

`/metrics` exposes `orion_traces_queue_spans`, `orion_traces_written_total`, `orion_traces_dropped_total` and `orion_traces_storage_full`.

## 4. Retention

Spans follow the app's settings, as logs do:

- `retention_days` of the app, but never longer than `trace_retention_days` in the data lifecycle settings (`PUT /v1/settings/data-lifecycle`, default 7, 1 to 3650). Spans are much larger than log lines, so the instance-wide limit is shorter than the app default of 14 days. Raise it to keep traces as long as logs.
- `max_storage_mb` of the app (0 = no cap) applies to its spans on their own, apart from its logs: when the spans exceed it, the oldest **whole traces** are removed first.

There is no cap on the number of spans of one trace: a trace of many thousands of spans is stored whole (reads return the first 5,000), and `max_storage_mb` is what limits it. Storage is bounded per span and per app, not per trace.

An hourly job prunes both in short batches, rebuilds the summary of any trace it cut, deletes summaries with no spans left, and returns freed pages to the disk. Deleting an app removes its spans, and spans of that app still waiting in the write queue are refused from then on (for an hour; app ids are not reused), so none is written after the delete. A span that was accepted over HTTP in the instant before the delete is not stored. Spans that do end up for an app that is missing from `orion.db` (a restore of an older `orion.db`, for instance) are not pruned by retention, which only looks at apps that exist. "Purge logs" on an app does not touch its traces.

## 5. API

All read routes need the admin login, like the log routes. Times are RFC 3339 or relative (`-1h`, `-7d`, `now`).

| Route | What it returns |
|---|---|
| `GET /v1/traces` | One row per trace and app, newest first, with a cursor. Filters: `app_id`, `service_id` (the apps linked to a service), `service` (`service.name` of the root span), `status` (`ok`, `error`), `min_duration` (`250ms`, `2s`, or milliseconds), `name` (substring of the root span name), `from`, `to`, `attr=key:value` (repeatable; matches when a span or the resource carries the value), `limit` (up to 200), `cursor`. `app_id` and `attr` take at most 50 values each (more is a 400). Without `to` the search runs up to five minutes past now, so spans stamped by a clock a little ahead still show; a span more than five minutes ahead (up to the one hour ingest allows) needs an explicit `to`. |
| `GET /v1/traces/{trace_id}` | Every span of the trace ordered by start (up to 5,000, then `truncated`), each with its offset from the trace start, and `log_count`: the product log entries that carry the trace id (`GET /v1/logs?trace_id=`). |
| `GET /v1/traces/facets` | For a filter bar: ok and error counts and the 50 most common services and root span names, for the same filters. Without `from`, the last 24 hours; the same 50-value limit and `to` default as the search. |
| `GET /v1/services/{id}/traces/summary?from=&to=` | Trace count, error count and rate, and the p50, p95 and p99 root-span duration of the traces the service's linked apps received (default the last hour, at most 7 days). Counts are exact; percentiles use the newest 50,000 traces that have a root (`sampled`). |

`status=error` means at least one span of the trace has status error. Searches stop after 10 seconds with a 504 `search_timeout`: narrow the time range or add a filter. The exact shapes are in `apps/core/openapi.yaml`.

## 6. Back up

`traces.db` is optional to back up. It holds only spans, which refill as apps keep sending; see [sqlite-backup-restore.md](sqlite-backup-restore.md).
