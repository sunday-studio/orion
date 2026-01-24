---
name: Fix all Orion improvements
overview: 'Address the 13 "what can be better" items: report.go debug/dead code, frontend API base and envelope handling, config/docs alignment, report-service logging and StoreMonitorReport logic, Dockerfile, and READMEs.'
todos:
  - id: report-go
    content: "report.go: remove fmt.Println, PrettyPrint, truncate rawData, drop fmt import"
    status: completed
  - id: custom-instance
    content: "custom-instance: getApiBase() returns /v1 when unset; use in customInstance and authLogin"
    status: completed
  - id: env-example
    content: "frontend/.env.example: 8080 to 8999"
    status: pending
  - id: core-readme-api
    content: "core/README: API under /v1, real report paths, link to openapi.yaml"
    status: pending
  - id: root-readme
    content: "Root README: overview, quick start, links"
    status: completed
  - id: scripts-readme
    content: "scripts/README: agent-install.sh as planned, not in repo"
    status: cancelled
  - id: report-service
    content: "report-service: Error (error,err), Info keys, StoreMonitorReport Error vs Metrics, remove comment block"
    status: completed
  - id: dockerfile-sdk
    content: "Dockerfile: remove COPY sdk/ sdk/ in frontend stage"
    status: pending
  - id: envelope-helper
    content: (Optional) dataOf helper and use in agent-detail, monitor-detail, home-page
    status: pending
  - id: tests
    content: (Optional) Add *_test.go in core/utils, config, api
    status: pending
isProject: false
---

# Plan: Fix All Orion Improvements

## 1. core/internal/api/report.go

**Changes:**

- **Remove** `fmt.Println("agentID", agentID)` at line 90. Replace with `s.logger.Debug("receiveMonitorReport", "agent_id", agentID)` (optional; or delete if debug noise is undesired).
- **Remove** `PrettyPrint` (lines 141–144) and the `"fmt"` import if it becomes unused.
- **Truncate** `rawData` in the unmarshal error at line 119: e.g. `trunc := string(rawData); if len(trunc) > 200 { trunc = trunc[:200] + "…" } `and log `"rawData", trunc` instead of full `rawData`.

---

## 2. Frontend API base when VITE_API_BASE_URL is unset

**File:** [frontend/src/lib/custom-instance.ts](frontend/src/lib/custom-instance.ts)

**Problem:** `base` is `""` when unset. Orval paths are `/agents`, `/monitors/:id`, etc. Core serves under `/v1`. With no `.env`, `fetch("/agents")` returns 404.

**Fix:**

- Introduce a single `getApiBase()` that returns `VITE_API_BASE_URL` (trimmed) when set, otherwise `"/v1"` for relative/SPA usage.
- Replace the top-level `const base = ...` with `getApiBase()` (or a `base` derived from it) in `customInstance`.
- In `customInstance`, when building `full`, use:

`const prefix = getApiBase();`

`const full = url.startsWith("http") ? url : \`${prefix}${url.startsWith("/") ? url : \`/\${url}\`}\`;`

so that when`prefix`is`"/v1"`, calls become `/v1/agents`, etc.

- Update `authLogin` to use the same `getApiBase()` (it already branches on `b`; ensure when `b` is `""` we use `"/v1"`). Currently `getApiBase()` returns `""` and `authLogin` does `b ? \`${b}/auth/login\` : "/v1/auth/login"`, so login is correct. After the change, `getApiBase()` returns `"/v1"` when env is unset, so `b` will be `"/v1"` and `\`${b}/auth/login\``=>`/v1/auth/login`. Good. For `customInstance`, `prefix`will be`"/v1"`, so `/v1`+`/agents`=>`/v1/agents`. Ensure no double slash: if `getApiBase()`returns`"/v1"`(no trailing slash) and`url`is`/agents`, we get `/v1/agents`. Good.
- **Edge:** when `VITE_API_BASE_URL` is set to e.g. `http://localhost:8999/v1`, `getApiBase()` returns that and `customInstance` will use it as-is. When unset, `getApiBase()` returns `"/v1"` so relative `/v1/agents` works when SPA is served from core.

---

## 3. frontend/.env.example

**Change:** `VITE_API_BASE_URL=http://localhost:8080/v1` → `VITE_API_BASE_URL=http://localhost:8999/v1` to match core default `ORION_PORT=8999`.

---

## 4. core/README.md — API Endpoints

**Current:** Documents `POST /register`, `POST /report/:agent_id`, `GET /health`; omits `/v1` and real agent report paths.

**Update:**

- State that the API is under `/v1` (e.g. `POST /v1/register`, `POST /v1/auth/login`, `GET /v1/agents`, `GET /health` unversioned).
- Replace `POST /report/:agent_id` with the actual routes from [core/internal/api/routes.go](core/internal/api/routes.go):
  - `POST /v1/agents/:agent_id/report` (agent report)
  - `POST /v1/agents/:agent_id/:monitor_id/report` (monitor report)
- Add a line: “See [core/openapi.yaml](core/openapi.yaml) for the full frontend and agent API.”

---

## 5. Root README.md

**Current:** Empty.

**Add:**

- Short project overview (Orion: Core + Agent; Core receives agent telemetry and serves a web UI).
- **Quick start:** run Core (`cd core && go build -o orion-core . && ./orion-core` or `make docker-up`), run Agent (point `core_url` in `agent/config.yaml` to Core), optional `make build-static` for the SPA.
- **Links:** [core/README.md](core/README.md), [agent/docs](agent/docs), [scripts/README.md](scripts/README.md), [docs/agent-core-contract.md](docs/agent-core-contract.md).

---

## 6. scripts/README.md

**Problem:** Documents `agent-install.sh` as if it exists; it does not.

**Fix:** In the “Installation Scripts” section, replace the `agent-install.sh` bullet with: “**agent-install.sh** (planned, not yet in repo) – Install and register the Orion agent (Linux/macOS, systemd/launchd).” Remove it from any “Usage” examples. Optionally add a one-line “Planned” subsection.

---

## 7. core/internal/service/report-service.go

**Logger usage (slog expects key-value pairs):**

- Line 79: `s.logger.Error("Failed to store monitor report", err)` → `s.logger.Error("Failed to store monitor report", "error", err)`.
- Line 133: `s.logger.Error("Failed to store agent report", err)` → `s.logger.Error("Failed to store agent report", "error", err)`.

**Info key format:**

- Line 112: `"agent_report_id ->"` → `"agent_report_id", agentReport.ID`.
- Line 113 (StoreMonitorReport): `"monitor_report_id ->"` → `"monitor_report_id", monitorReport.ID`.

**StoreMonitorReport payload (lines 44–68):**

If `payload.Error != nil`, `payloadData` is set from `payload.Error`; then `payloadData = string(payloadJSON)` from `json.Marshal(payload.Metrics)` always runs and overwrites it. When `health` is down and `Error` is set, we should store the error, not Metrics.

**Fix:**

- Use `if payload.Error != nil { ... payloadData = string(payloadJSON) } else { ... payloadData = string(metricsJSON) }` so only one branch sets `payloadData`.
- **Remove** the commented block (lines 48–54) or leave a one-line `// when health=down we could store full payload` if you want to preserve the idea.

---

## 8. core/Dockerfile — frontend stage

**Change:** Remove `COPY sdk/ sdk/` from the frontend stage. The frontend build uses `frontend/src/lib/api.ts` (Orval-generated from `core/openapi.yaml` at build time elsewhere); it does not import `sdk/`. The `sdk/` copy is unused in this stage.

---

## 9. (Optional) Frontend envelope helper

**Goal:** Reduce repeated `res?.data?.data?.x` and avoid typos.

**Add** in [frontend/src/lib/custom-instance.ts](frontend/src/lib/custom-instance.ts) or a small `frontend/src/lib/envelope.ts`:

```ts
export function dataOf<T>(
  r: { data?: { data?: T } } | null | undefined,
): T | null {
  return r?.data?.data ?? null;
}
```

**Use** in:

- [frontend/src/pages/agent-detail-page.tsx](frontend/src/pages/agent-detail-page.tsx): e.g. `const agent = dataOf(detailRes)?.agent ?? null`, and for `latest_report`, `monitors`, `reports`, `count`, `uptime` from the respective hooks (adjust for shapes that are `{ agent, latest_report }`, `{ monitors }`, etc.; a generic `dataOf` fits `{ data: { data: X } }` and for nested like `{ agent, latest_report }` the callers still do `dataOf(detailRes)?.agent`).
- [frontend/src/pages/monitor-detail-page.tsx](frontend/src/pages/monitor-detail-page.tsx): `dataOf(detailRes)?.monitor`, `dataOf(detailRes)?.recent_reports`, `dataOf(historyRes)?.reports`, `dataOf(historyRes)?.count`, `dataOf(uptimeRes)` for uptime.
- [frontend/src/pages/home-page.tsx](frontend/src/pages/home-page.tsx): `dataOf(agentsRes)?.agents`, `dataOf(agentsRes)?.count`, `dataOf(monitorsRes)?.monitors`.

This is an optional refactor; the current `res?.data?.data?.x` is correct.

---

## 10. (Optional) Tests

**Add** a few `*_test.go` under `core/`:

- `core/internal/utils/response_test.go`: table-driven tests for `SuccessResponse` / `ErrorResponse` (via `httptest.ResponseRecorder` and a Gin context).
- `core/internal/config/config_test.go`: `Load` with env set/unset, `Validate` when frontend auth is on and `ORION_JWT_SECRET` is empty vs set.
- `core/internal/api/report_test.go` or `agent_test.go`: one or two handlers with a test DB (e.g. in-memory SQLite) to lock in request/response shape.

Treat as lower priority; can be done after the code and docs fixes above.

---

## Execution order

| Order | Task | Files |

| ----- | ----------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |

| 1 | report.go: remove `fmt.Println`, `PrettyPrint`, truncate `rawData`, drop `fmt` if unused | [core/internal/api/report.go](core/internal/api/report.go) |

| 2 | custom-instance: `getApiBase()` returning `"/v1"` when unset, use in `customInstance` and `authLogin` | [frontend/src/lib/custom-instance.ts](frontend/src/lib/custom-instance.ts) |

| 3 | .env.example port 8080 → 8999 | [frontend/.env.example](frontend/.env.example) |

| 4 | core README: API under `/v1`, real report paths, link to openapi.yaml | [core/README.md](core/README.md) |

| 5 | Root README: overview, quick start, links | [README.md](README.md) |

| 6 | scripts README: agent-install.sh as “planned” | [scripts/README.md](scripts/README.md) |

| 7 | report-service: Error args, Info keys, StoreMonitorReport Error vs Metrics, remove comment block | [core/internal/service/report-service.go](core/internal/service/report-service.go) |

| 8 | Dockerfile: remove `COPY sdk/ sdk/` in frontend stage | [core/Dockerfile](core/Dockerfile) |

| 9 | (Optional) `dataOf` helper and use in agent-detail, monitor-detail, home-page | [frontend/src/lib/custom-instance.ts](frontend/src/lib/custom-instance.ts) or new `envelope.ts`, and the three pages |

| 10 | (Optional) `*_test.go` in core | `core/internal/utils`, `core/internal/config`, `core/internal/api` |

---

## Notes

- **Makefile:** `docker-build` and `docker-up` are already present; no change.
- **core/README:** Build, port 8999, Docker, and config table are already up to date; only the API Endpoints section and the openapi link need edits.
- **AgentDetailPage:** Implemented in [frontend/src/pages/agent-detail-page.tsx](frontend/src/pages/agent-detail-page.tsx); no restore needed.
