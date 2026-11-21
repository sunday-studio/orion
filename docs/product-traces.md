# Product Traces

How an app sends traces to Orion and how Core stores and serves them. Plan: [plans/opentelemetry-support.md](plans/opentelemetry-support.md), phase 3. Logs are in [product-logs.md](product-logs.md) and metrics in [product-metrics.md](product-metrics.md).

Status: **live**: OTLP/HTTP and OTLP/gRPC ingest, storage, retention, the read API and the Console trace views (section 6). The setup for all three signals is in [opentelemetry.md](opentelemetry.md).

## 1. Connect an app

Point the OTLP/HTTP traces exporter at Orion with the app's ingest key (the same key as logs and metrics, from **Logs > Apps**). The guide [opentelemetry.md](opentelemetry.md) has the shorter setup that covers all three signals at once, and examples for Node.js, Python, Go, Java and the Collector. For traces alone:

```sh
export OTEL_EXPORTER_OTLP_TRACES_ENDPOINT="$ORION_URL/v1/ingest/otlp/v1/traces"
export OTEL_EXPORTER_OTLP_TRACES_HEADERS="Authorization=Bearer%20$ORION_INGEST_KEY"
export OTEL_EXPORTER_OTLP_TRACES_PROTOCOL="http/json"   # http/protobuf also works
export OTEL_SERVICE_NAME="checkout-api"
```

Many SDKs default to OTLP/gRPC instead. Core has an opt-in gRPC receiver on port 4317 (`ORION_OTLP_GRPC_ADDR`), with the same key, limits and rate limit: `OTEL_EXPORTER_OTLP_ENDPOINT=http://host:4317`, `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` and `OTEL_EXPORTER_OTLP_HEADERS="authorization=Bearer%20<ingest key>"`. Setup, status codes and limits: [otlp-grpc.md](otlp-grpc.md).

Link the app to a [service](plans/service-correlation-plan.md) and the service's traces, error rate and latency come with it. An OpenTelemetry Collector works the same way: an `otlphttp` exporter with `traces_endpoint` and an `Authorization` header.

## 2. What is stored

Core stores every span as sent, in `traces.db` (next to `orion.db` and `logs.db` in `ORION_DATA_DIR`): ids, parent, name, kind, start and duration in nanoseconds, status, attributes, the resource, events and links. Scope name and version are kept as the `otel.scope.name` and `otel.scope.version` attributes. The resource's `service.name` is also kept in its own column for filtering.

Core also keeps one **trace summary** per app and trace id, maintained by the writer: root span name, service, start, duration, span count, error count and whether the root has arrived.

- A trace is whatever arrived. Spans of one trace may come in any order and in separate requests (a child is usually exported before its parent). Orphan spans, whose parent never arrives, are kept.
- The summary is rebuilt from the stored spans every time a batch touches the trace, so it is the same whatever the arrival order. While no root span has arrived, the summary is named after the earliest span, its duration is the extent of the spans, and `has_root` is false; the root, when it arrives, takes over.
- The root is the earliest span with no parent. The trace duration is the root span's duration.
- Sending the same span twice (an exporter retry) stores it once and counts it once.
- In OTLP/JSON the trace, span and parent ids are hex strings, as the OTLP specification says (`"traceId":"5b8efff798038103d269b633813fc60c"`); Core also reads the base64 form of the protobuf JSON mapping. Spans and span links are stored with hex ids either way.
- A trace id seen by two apps (both sides of a call are instrumented, each with its own key) is two summaries, one per app, each with that app's spans. `GET /v1/traces/{trace_id}` returns the spans of all apps.

## 3. Limits, errors and retries

| Limit | Value |
|---|---|
| Spans per request | 10,000 (413 beyond) |
| Request body | 5 MiB, also after gzip decompression (413 beyond) |
| One span | 256 KiB, measured as stored (every column, the resource included) and not only as sent; larger spans are refused |
| Attributes per span | 128, the first ones in the order sent; the rest are dropped |
| Resource attributes | 128 and 8 KiB of encoded JSON on each span; service, host, `k8s.*` and `cloud.*` keys are kept first, then the rest in key order. The resource is stored with every span, so a request of 10,000 spans can store up to 80 MB of it; the per-app storage cap counts those bytes |
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
| `GET /v1/traces` | One row per trace and app, newest first, with a cursor. Filters: `app_id`, `service_id` (the apps linked to a service), `service` (`service.name` of the root span), `status` (`ok`, `error`), `min_duration` (`250ms`, `2s`, or milliseconds), `name` (substring, any case, of the name the list shows: the root span name, or the earliest span's name while the root has not arrived), `from`, `to`, `attr=key:value` (repeat it for several pairs, for example `attr=http.status_code:500&attr=db.system:postgres`; a trace must match every pair, and a pair matches when a span or the resource carries the value), `limit` (up to 200), `cursor`. `app_id` and `attr` take at most 50 values each (more is a 400). Without `to` the search runs up to five minutes past now, so spans stamped by a clock a little ahead still show; a span more than five minutes ahead (up to the one hour ingest allows) needs an explicit `to`. |
| `GET /v1/traces/{trace_id}` | Every span of the trace ordered by start (up to 5,000, then `truncated`), each with its offset from the trace start, and `log_count`: the product log entries that carry the trace id (`GET /v1/logs?trace_id=`). |
| `GET /v1/traces/facets` | For a filter bar, with the same filters as the search. `total` is the number of traces that match every filter, `status` is the ok and error counts of the traces that match every filter except `status` (so a filter bar can show both choices), and `services` and `names` are the 50 most common values among the traces that match every filter. Without `from`, the last 24 hours; the same 50-value limit and `to` default as the search. |
| `GET /v1/services/{id}/traces/summary?from=&to=` | Trace count, error count and rate, and the p50, p95 and p99 root-span duration of the traces the service's linked apps received (default the last hour, at most 7 days). Counts are exact; percentiles use the newest 50,000 traces that have a root (`sampled`). |

`status=error` means at least one span of the trace has status error. Searches stop after 10 seconds with a 504 `search_timeout`: narrow the time range or add a filter. The exact shapes are in `apps/core/openapi.yaml`.

## 6. In the Console

Open **Logs, Traces** (or press `G` then `T`).

- **Traces list.** One row per trace and app: root span, service, a duration bar scaled to the slowest trace on the page, span count, errors and start time. A trace whose root span has not arrived is marked *partial* and named after its earliest span. Filters are in the URL, so a view can be shared: search by name, service, app, status, minimum duration, time range and attributes. Attributes are `key:value` pairs: type one and press Enter, repeat to add more (a trace must match all of them), and remove one with its chip. Status counts and the match count come from the facets, and paging is by cursor.
- **Trace page.** A waterfall with the span tree, the time axis and the failed spans marked, a panel for the selected span (attributes, resource, events, links, stack trace, and a link that finds other traces with the same attribute value), and the log lines that carry the trace id, grouped by span. **View logs** opens them in the Logs screen.
- **Links.** A trace id in a log entry or an incident opens its trace. A service page shows its traces, error rate and latency (p50, p95, p99) from the root spans of its linked apps, and **Open in Traces** opens the list for that service. Histogram metrics appear on the service page as p50, p95 and p99 lines.

The pages read the API above and need an admin login. Without traces enabled on Core they say so instead of showing an empty list.

## 7. Back up

`traces.db` is optional to back up. It holds only spans, which refill as apps keep sending; see [sqlite-backup-restore.md](sqlite-backup-restore.md).
