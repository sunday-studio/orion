---
title: API overview
description: "Authentication, ingest endpoints and where to find the full OpenAPI spec."
group: Reference
order: 9
---
Core exposes a JSON API under `/v1`. The complete contract is the OpenAPI document in the repository: [`apps/core/openapi.yaml`](https://github.com/sunday-studio/orion/blob/main/apps/core/openapi.yaml).

## Authentication

| Caller | Credential |
|---|---|
| Console and admin API | Sign in with the admin account, then use the session token. |
| Apps sending logs | An ingest key: `Authorization: Bearer orion_ik_...`. Write-only, for one app. |
| Nodes | A per-Node token issued at registration. |

An ingest key cannot read logs or touch monitors, and a Node token cannot ingest as an app.

## Common endpoints

| Endpoint | Purpose |
|---|---|
| `GET /health` | Liveness. |
| `GET /v1/diagnostics/core-worker` | Whether the worker is alive. |
| `POST /v1/ingest/logs` | Send logs (JSON, array or NDJSON). |
| `POST /v1/ingest/otlp/v1/logs` | Send logs over OTLP/HTTP. |
| `GET /v1/logs` | Search logs. |
| `GET /v1/logs/stream` | Live tail over Server-Sent Events. |
| `GET /v1/incidents/{id}/logs` | Logs around an incident. |

See [Send logs](/docs/send-logs/) for payloads, limits and status codes.
