# Product Logs: Plan

Status: proposed, awaiting approval. 2026-10-17.
Related: [backend-hardening-e2e-plan.md](backend-hardening-e2e-plan.md), [decision-log.md](decision-log.md), `docs/design/implementation-handoff.md`.

## 1. Goal

Any app, including open-source ones we do not control, can send **product logs** to Orion and see them in one place. Product logs means what the app says about itself: "user signed up", "payment failed", "export finished", "job retried 3x", errors with context. Not only network or system logs.

Constraints from the user:
- No required structure. Open-source apps will not adopt a vendor-specific schema.
- Display is still uniform: one page, same look for every app.

## 2. What exists today

Orion already stores `service_log_entries` (`apps/core/internal/db/models.go:144`), ingested only from **agents** via `POST /:agent_id/logs/batch` with an agent token. Gaps for this goal:
- Identity is an agent, not an application. An open-source app is not a server we monitor.
- Entry shape is agent-centric (`monitor_id`, `stream`, required client `fingerprint`).
- No full-text search, facets, live tail, or per-app retention (ORB-026 already flags no pruning).
- `service_log_entries` also feeds the agent's own diagnostics. Mixing product logs into it would blur that.

Decision: product logs get their own ingest path and tables. Agent service logs stay as they are and are shown in the same UI as one source type.

## 3. Design

### 3.1 The uniform envelope (small, forgiving)

Align with the **OpenTelemetry log data model** rather than inventing one. That is what open-source maintainers already know, and it gives free interop.

```
timestamp    optional, server fills if missing
severity     trace|debug|info|warn|error|fatal (accepts OTel numbers and common strings)
body         string, or any JSON (stored as text plus parsed form)
event        optional short name, e.g. "payment.failed" (the product-event hook)
attributes   free-form JSON object, any keys, any depth
resource     optional: service.name, service.version, deployment.environment, host
trace_id / span_id / request_id   optional correlation
id           optional client id for idempotency
```

Rules: only `body` or `event` is required. Unknown fields go to `attributes`. Nothing else is mandatory. A plain `console.log` line is a valid entry.

### 3.2 Ways in (pick the one the app already speaks)

| Path | For | Notes |
|---|---|---|
| `POST /v1/ingest/logs` native JSON or NDJSON | Anything, `curl`, shell, simple apps | The documented minimum |
| `POST /v1/ingest/otlp/v1/logs` (OTLP/HTTP JSON and protobuf) | Apps using any OpenTelemetry SDK or the OTel Collector | Zero Orion-specific code needed in open-source apps |
| Tiny clients (TypeScript, Go, Python), about 100 lines each | Convenience: batching, retry, flush on exit | Optional; never required |
| Agent tail (later) | Existing log files and container stdout on monitored hosts | Reuses the agent; maps lines to the envelope |

### 3.3 Applications and keys

New resource: **App** (`id`, `name`, `slug`, `environment`, retention, sample rate, display profile). Each App gets one or more **ingest keys**: write-only, hashed at rest, rotatable, shown once. Separate from agent tokens, so a leaked app key cannot read logs or touch monitors. Browser-side logging is out of scope initially (a public key needs origin allowlists and abuse controls).

### 3.4 Uniform display without a fixed schema

The trick is to normalize **at read time**, not at write time:
1. **Well-known fields** render specially when present: severity, event, `error.*`, `user.id`, `request.id`, `http.*`, trace id (click to filter).
2. **Field catalog**: Core records, per app, which attribute keys exist, their dominant type, and rough cardinality. This powers facets and column choices with no schema.
3. **Display profile per app** (optional): the user picks which attribute is the "title", which are columns, and which are hidden or redacted. Defaults are automatic; the profile is a nicety, not a requirement.
4. **Generic fallback**: everything else is a collapsible key-value tree in the detail drawer.

Result: every app looks the same in the list, and apps with richer data get richer facets automatically.

### 3.5 Storage and scale (SQLite, per D-001)

- Separate database file `logs.db` (WAL, attached or opened separately) so log write volume does not contend with incidents/monitors and so retention `VACUUM` stays isolated.
- Table `log_entries`: id, app_id, ts, severity, event, body, attributes (JSON), resource (JSON), trace_id, request_id, ingested_at. Indexes on `(app_id, ts)`, `(app_id, severity, ts)`, `(app_id, event, ts)`. FTS5 virtual table over `body` and flattened attributes.
- Ingest is async: validate, enqueue to a bounded in-process queue, write in batched transactions, respond `202`. Backpressure returns `429` with `Retry-After`.
- Limits: body 64 KiB per entry, 1 MiB per request (5 MiB for NDJSON), attribute depth and key-count caps.
- Retention per app (default 14 days, plus a size cap per app), pruned in batches (reuses the batching from backlog item ORB-027).
- Per-app rate limit and optional sampling for debug level.
- Server-side redaction rules (reuse the existing redaction package): default-redact common secret keys (`password`, `authorization`, `token`, etc.). Important for open-source apps whose users did not choose to send secrets.

### 3.6 Console

One **Logs** page showing all sources (apps, agents) with a source filter:
- OpenStatus-style data-table (AGENTS.md rule): server-side filtering, sorting, pagination, nuqs URL state.
- Volume histogram, severity and event facets, field facets from the field catalog, free-text search, relative time picker.
- Live tail (SSE) with pause, saved views, share-by-URL.
- Detail drawer (already built): attribute tree, copy as JSON, jump to trace/request, "related logs".
- Settings: Apps list, create app, ingest keys, retention, display profile, redaction rules, "send a test log" with copy-paste snippets (curl, OTel env vars, TS, Go, Python).

### 3.7 Connection to the rest of Orion (phase 3)

- Link logs to incidents by app plus time window ("logs around this incident").
- Log-based alerts through the existing outbox/dispatcher: "more than N errors in 5 minutes", "event X never seen in an hour" (reuses the heartbeat idea). Webhook and Email only (D-008).

## 4. Phases

| Phase | Scope | Depends on | Done when |
|---|---|---|---|
| L0 | Contract: envelope spec, OpenAPI for ingest, doc in `docs/agent-core-contract.md` and a public `docs/product-logs.md` for app authors | none | Reviewed spec; sample payloads |
| L1 | Core ingest: apps, ingest keys, native endpoint, async queue, `logs.db`, limits, redaction, retention, rate limits | Hardening Phase 0 (WAL profile, testkit) | A `curl` entry shows up via API; load test passes |
| L2 | Console: unified Logs page, filters, FTS search, histogram, field catalog and facets, apps and keys settings, snippets | L1, design for the page | Usable end to end for one real app |
| L3 | OTLP endpoint, live tail SSE, saved views, display profile, TS/Go/Python clients | L1 | An app using a stock OTel SDK needs no Orion code |
| L4 | Incident linking, log-based alerts, agent file/stdout tail | Hardening outbox (WP 1.1), L2 | Alert fires once on an error burst |

Slots in after hardening Phases 0-1 so it lands on the WAL profile, test kit, and outbox. Design for L2 can start now. It also absorbs the earlier backlog items "logs query/histogram/SSE/patterns/export" (D-009).

## 5. Tests and gates

- **Ingest:** plain string body, nested JSON, NDJSON batch, OTLP JSON and protobuf, oversized entry (rejected, batch survives), duplicate `id` (idempotent), clock skew, unknown severity.
- **Isolation:** key for app A cannot write to app B; ingest key cannot call read APIs; agent token cannot ingest as an app.
- **Safety:** secrets redacted, huge attribute trees capped, FTS injection characters handled, no `database is locked` under load.
- **Performance targets (to confirm in L1):** sustained 1,000 entries/s with p95 ingest under 100 ms; list query with filters under 300 ms on 10M rows; retention prune never blocks ingest for more than 1 s.
- **Soak:** 72h with three apps, one noisy; DB size plateaus at retention.
- **UI:** Playwright for filter-by-facet, search, live tail pause, URL state restore.
- Each phase ends with evidence and independent review (AGENTS.md).

## 6. Decisions I would make (for review)

| # | Decision | Alternative rejected |
|---|---|---|
| P1 | OTel log data model as the envelope | Custom schema (adoption barrier for open source) |
| P2 | New ingest keys per App, not agent tokens | Reuse agent tokens (over-privileged, wrong identity) |
| P3 | Read-time normalization with field catalog and display profile | Require fields at write time |
| P4 | Separate `logs.db` SQLite file | Same DB as incidents (contention, VACUUM impact) |
| P5 | Async `202` ingest with bounded queue and backpressure | Synchronous writes (blocks apps, repeats ORB-005) |
| P6 | Browser-side logging deferred | Public keys now (abuse surface) |
| P7 | Server-side default redaction on | Opt-in redaction |

## 7. Risks

| Risk | Mitigation |
|---|---|
| Log volume overwhelms single-writer SQLite | Separate file, batched writes, per-app rate limits and caps; measure in L1 before building UI. If the target is missed, add write-sharding by app before considering another engine |
| "No structure" turns the UI into mush | Field catalog, well-known field rendering and display profiles; usability test with two real apps |
| Secrets in logs | Default redaction, size limits, per-app rules, clear docs |
| Scope creep into a full observability suite | Out of scope: traces and metrics storage, dashboards builder, long-term archive. Logs and log-based alerts only |
