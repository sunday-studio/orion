# Send logs to Orion

Status: **L1 is live**: apps, ingest keys, `POST /v1/ingest/logs` (JSON, array, NDJSON), `GET /v1/logs` and `GET /v1/logs/{id}` work. OTLP, the histogram, field catalog, saved views and live stream are specified in `apps/core/openapi.yaml` and answer `501 Not Implemented` until their phase ships; shapes will not change incompatibly. Design: [plans/product-logs-plan.md](plans/product-logs-plan.md).

Orion collects *product logs*: what your app says about itself ("user signed up", "payment failed", "export finished", errors with context). There is no required schema. A plain message is a valid log; richer data gets richer filtering automatically.

## 1. Get an ingest key

In Orion: Settings, Apps, create an app, then create an ingest key. The key looks like `orion_ik_...`, is shown **once**, and can only write logs for that one app (it cannot read logs or touch monitors). To rotate: create a second key, deploy it, revoke the old one.

Over the API (admin login required): `POST /v1/apps`, then `POST /v1/apps/{id}/keys`.

## 2. Send a log

```sh
curl -X POST "$ORION_URL/v1/ingest/logs" \
  -H "Authorization: Bearer $ORION_INGEST_KEY" \
  -H "Content-Type: application/json" \
  -d '{"severity":"error","event":"payment.failed","body":"payment failed for order 8841","attributes":{"order_id":8841,"amount":4200,"currency":"EUR"}}'
```

Response `202 Accepted`:

```json
{"accepted": 1, "rejected": 0}
```

`202` means queued. Entries become searchable within about a second. The smallest valid request is `{"body":"hello"}`.

### Batching

Send an array, or newline-delimited JSON with `Content-Type: application/x-ndjson`:

```sh
printf '%s\n' '{"body":"job started","attributes":{"job":"export"}}' '{"body":"job finished","attributes":{"job":"export","rows":120}}' |
curl -X POST "$ORION_URL/v1/ingest/logs" -H "Authorization: Bearer $ORION_INGEST_KEY" -H "Content-Type: application/x-ndjson" --data-binary @-
```

Entries are checked one by one. A bad entry is reported in `errors` (with its `index` and a `code`) and the rest are still accepted:

```json
{"accepted": 98, "rejected": 2, "errors": [{"index": 3, "code": "entry_too_large", "message": "entry exceeds 65536 bytes"}]}
```

## 3. The entry format

Every field is optional, but each entry needs a `body` or an `event`.

| Field | Type | Meaning |
|---|---|---|
| `body` | string, or any JSON | The message. Structured bodies are kept and searchable as text. |
| `event` | string | Short stable name for product events, for example `payment.failed`. Lets you filter and alert on an event. |
| `severity` | string or number | `trace`, `debug`, `info`, `warn`, `error`, `fatal`. Also accepts OpenTelemetry numbers 1-24 and aliases (`warning`, `err`, `critical`, ...). Unknown values become `info`. |
| `timestamp` | string or number | RFC 3339 text, or Unix seconds, milliseconds or nanoseconds. Defaults to the time Orion receives it. |
| `attributes` | object | Anything, any shape. Limits: depth 8, 128 keys. |
| `resource` | object | Who emitted it: `service.name`, `service.version`, `deployment.environment`, `host.name`. |
| `trace_id`, `span_id`, `request_id` | string | Correlation ids. Clicking one filters to related logs. |
| `id` | string | Your own unique id (per app). Re-sending the same id is ignored, so retries are safe. |

Any other top-level field is stored as an attribute, so you can send the JSON your logger already produces.

Well-known attribute names get special treatment in the UI when present: `error.message`, `error.stack`, `user.id`, `http.method`, `http.status_code`, `http.route`, `request.id`.

## 4. Use OpenTelemetry instead

Apps that already use an OpenTelemetry SDK (or the OTel Collector) need no Orion code. Point the OTLP/HTTP logs exporter at Orion:

```sh
export OTEL_EXPORTER_OTLP_LOGS_ENDPOINT="$ORION_URL/v1/ingest/otlp/v1/logs"
export OTEL_EXPORTER_OTLP_LOGS_HEADERS="Authorization=Bearer $ORION_INGEST_KEY"
export OTEL_EXPORTER_OTLP_LOGS_PROTOCOL="http/json"   # http/protobuf also works
export OTEL_SERVICE_NAME="checkout-api"
```

OTLP log records map to Orion entries: `body`, `severityNumber`/`severityText`, `timeUnixNano`, `attributes`, `traceId`, `spanId`; the `resource` attributes become `resource`.

## 5. Limits, errors and retries

| Limit | Value |
|---|---|
| Entry size | 64 KiB |
| Request size | 1 MiB (JSON), 5 MiB (NDJSON), at most 10,000 entries (413 beyond that) |
| Attributes | depth 8, 128 keys per entry |
| Retention | 14 days by default, per app; also capped by a per-app storage limit |

| Status | Meaning | What to do |
|---|---|---|
| `202` | Accepted (check `rejected`/`errors` for per-entry problems) | nothing |
| `400` | Request is not valid JSON/NDJSON | fix the payload |
| `401` | Missing, wrong or revoked ingest key | check the key |
| `413` | Request too large | send smaller batches |
| `415` | Unsupported content type | use JSON, NDJSON or OTLP |
| `429` | App rate limit or ingest queue full; has `Retry-After` | wait that many seconds, then retry the same batch |
| `5xx` | Orion is unavailable | retry with exponential backoff |

Retry only on `429` and `5xx`. `debug_sample_rate` per app keeps that fraction of `trace` and `debug` entries (0 drops them all, 1 keeps all); sampled-out entries still count as accepted. Set a client `id` on entries if duplicates would matter.

## 6. Secrets

Orion redacts the values of common secret keys before storing anything: `password`, `passwd`, `secret`, `token`, `authorization`, `api_key`, `apikey`, `cookie`, `set-cookie`, `private_key`, `credit_card` (case-insensitive, at any depth, in attributes and structured bodies). Add more key names per app in its redaction settings. Redaction is a safety net; do not log secrets on purpose. Know its limits:

- It matches **key names** by substring, so harmless keys such as `tokens_used` or `max_tokens` are redacted too; rename them or turn the defaults off per app (`redaction.disable_defaults`) and list your own keys.
- It does not scan plain-text message bodies. A secret inside `"body": "login with password hunter2"` is stored as sent.
- Storage caps are approximate: an entry's size is counted from the bytes received, not the bytes stored.

## 7. Reading logs (Console and API)

These need an admin login, not an ingest key:

| Endpoint | Purpose |
|---|---|
| `GET /v1/logs` | Search across apps and agent service logs. Filters: `app_id`, `source`, `severity`, `event`, `q` (full text), `from`, `to`, `trace_id`, `request_id`, `attr=key:value`. Cursor pagination. |
| `GET /v1/logs/{id}` | One entry |
| `GET /v1/logs/histogram` | Volume over time by severity |
| `GET /v1/logs/fields?app_id=` | Attribute keys seen for an app, with types and cardinality (powers facets) |
| `GET /v1/logs/stream` | Live tail (Server-Sent Events) |
| `GET/POST/DELETE /v1/logs/views` | Saved filter sets |
| `/v1/apps`, `/v1/apps/{id}`, `/v1/apps/{id}/keys`, `/v1/apps/{id}/test-log` | Manage apps, keys, send a test log |

Rollout: **L1 (now)** apps, keys, native ingest, `GET /v1/logs`, `GET /v1/logs/{id}`, retention, rate limits. **L2** histogram, field catalog, saved views and agent logs in the same list (`source=agent`). **L3** OTLP, live stream. Search `q` is a substring match over the body, event and attributes today; full-text ranking arrives with L2.

Operational notes: logs live in their own `logs.db` next to `orion.db` (WAL, owner-only). Ingest is asynchronous: `202` means queued in memory (up to `ORION_LOGS_QUEUE_MAX` entries) and written within about a second; a hard crash can lose entries still in the queue. Retention (default 14 days, plus an optional per-app storage cap) is pruned hourly in small batches.
