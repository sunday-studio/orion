---
name: Docker and Frontend Auth
overview: "Add a multi-stage Docker build for the core service that includes the frontend, configurable data/port via env, and optional frontend auth: when ORION_ADMIN_USERNAME and ORION_ADMIN_PASSWORD are set at runtime, protect the app with a login form and JWT."
todos:
  - id: config
    content: Add core/internal/config and wire ORION_DATA_DIR, ORION_PORT, ORION_ADMIN_*, ORION_JWT_SECRET
    status: completed
  - id: db-datadir
    content: Update db.Initialize to accept dataDir and use it for SQLite path
    status: completed
  - id: backend-login-jwt
    content: Implement POST /v1/auth/login and FrontendAuthMiddleware (JWT), add jwt lib
    status: completed
  - id: routes-auth
    content: Register /v1/auth/login and apply FrontendAuthMiddleware to frontend v1 group
    status: completed
  - id: frontend-login
    content: Add LoginPage, /login route, and auth guard/401 handling in App and api.ts
    status: completed
  - id: dockerfile
    content: "Add core/Dockerfile (multi-stage: frontend, core, runtime)"
    status: completed
  - id: compose-makefile
    content: Add docker-compose and Makefile docker-build/docker-up
    status: completed
  - id: docs
    content: Document Docker build/run and env vars in README
    status: completed
isProject: false
---

# Docker Build, Deploy, and Frontend Auth for Orion Core

## Current State

- **Core** ([core/main.go](core/main.go)): Go + Gin, listens on `:8999`, serves API at `/v1/*` and SPA from `web/` (index.html + `/assets/*`). SQLite at `data/orion.db` (hardcoded in [core/internal/db/db.go](core/internal/db/db.go)).
- **Frontend** ([frontend/](frontend/)): Vite + React, built with `npm run build`; `make build-static` copies `frontend/dist/*` to `core/web/`. API base is `/v1` (or `VITE_API_BASE_URL`). No auth; [routes.go](core/internal/api/routes.go) marks frontend routes as "no auth for now."
- **Auth today**: Only agent token auth ([core/internal/api/auth.go](core/internal/api/auth.go), [core/internal/service/auth_service.go](core/internal/service/auth_service.go)) for agent-to-core. No user/admin model in [core/internal/db/models.go](core/internal/db/models.go).
- **Docker**: None.

---

## 1. Config and Env in Core

Introduce a small config module so the core reads env at startup.

**New:** `core/internal/config/config.go`

- `ORION_DATA_DIR` (default `"data"`) — directory for SQLite; `db.Initialize` will use it.
- `ORION_PORT` (default `"8999"`) — HTTP listen address.
- `ORION_ADMIN_USERNAME`, `ORION_ADMIN_PASSWORD` — optional; when **both** set, frontend auth is enabled (login form + JWT).
- `ORION_JWT_SECRET` — required when frontend auth is on; used to sign/verify JWTs. If both admin vars are set and this is empty, log a fatal or refuse to start (avoid unsafe default).

**Wire-up**

- [core/main.go](core/main.go): Load config, pass `DataDir` into `db.Initialize`, pass `Port` into `server.Start`, and pass an `AuthConfig` (or flags) into `api.NewServer` / `setupRoutes` so the server knows whether to enable frontend auth and which secret to use.

**DB**

- [core/internal/db/db.go](core/internal/db/db.go): `Initialize(dataDir string)` (or `Initialize(cfg *config.Config)`). Use `filepath.Join(dataDir, "orion.db")` instead of a fixed `data/orion.db`. `Migrate` stays as is (no new tables for admin; credentials are env-only).

---

## 2. Backend: Login and JWT Middleware for Frontend

**Dependencies**

- Add a JWT library, e.g. `github.com/golang-jwt/jwt/v5`, to [core/go.mod](core/go.mod).

**New:** `core/internal/api/auth_frontend.go` (or extend `auth.go`)

- **`POST /v1/auth/login`**
  - Body: `{"username","password"}`.
  - Compare to `ORION_ADMIN_USERNAME` and `ORION_ADMIN_PASSWORD` (constant-time).
  - If match: issue a JWT (e.g. `sub=username`, `exp=24h`), signed with `ORION_JWT_SECRET`; return `{"token":"<jwt>"}`.
  - If not match or auth disabled: 401.

- **`FrontendAuthMiddleware(secret string, enabled bool)`**
  - If `!enabled`: `c.Next()` (no-op).
  - If `enabled`: read `Authorization: Bearer <token>`, verify JWT with `secret`; on failure or missing -> 401.
  - Do **not** apply to: `/health`, `/v1/register`, `/v1/auth/login`, or to agent-only routes that use `AuthMiddleware` + `ValidateAgentToken`.

**Routes**

- In [core/internal/api/routes.go](core/internal/api/routes.go):
  - Register `POST /v1/auth/login` (public).
  - Apply `FrontendAuthMiddleware` only to the **frontend** `v1` group (agents, monitors, health/summary, health/issues, incidents/candidates, etc.). Leave `/v1/register` and agent-protected routes unchanged.

---

## 3. Frontend: Login Page and Token in API

**Login page**

- New: `frontend/src/pages/LoginPage.tsx`
  - Form: username, password; submit -> `POST /v1/auth/login` with `fetch` (or via a small `api.authLogin` helper).
  - On success: store `token` in `localStorage` (and/or in-memory), then `navigate("/")`.
  - On 401: show error (e.g. "Invalid credentials").

**Routing and guard**

- [frontend/src/App.tsx](frontend/src/App.tsx):
  - Add route `path="/login"` -> `LoginPage`.
  - Add an `AuthGuard` (or inline logic): when visiting `/`, `/agents/:id`, `/monitors/:id`, if there is no token in `localStorage`, redirect to `/login`.
  - If there is a token, render the existing routes as today.
  - When auth is **disabled** (backend returns 200 for frontend routes without a token), the guard must still allow access. Simple approach: **always** send `Authorization: Bearer <token>` when token exists; if no token, still try to load. Guard: if we get 401 on any API call, clear token and redirect to `/login`. That way, when backend has auth off, no token is fine; when auth is on, 401 forces login.
  - To avoid flash of protected content: on app load, you can do a cheap `GET /v1/health/summary` or `GET /v1/agents?limit=1`; on 401, redirect to `/login` and do not render children. If you prefer to avoid an extra request, the guard can simply "if no token and we want to be strict, redirect to /login" — but then with auth disabled there is no token, so we’d need a way to know that auth is off. **Pragmatic choice:**
    - Guard checks for token only when we want to protect.
    - Since we can’t know from the frontend alone if auth is on, use: **if no token, allow rendering**; **on 401 from any `api.*` call, clear token and redirect to `/login`.**
    - For the first load: user with no token hits `/`; app loads, fetches agents (or similar); if 401, then redirect to `/login`. No extra /auth/status call.
  - Optional: a dedicated `GET /v1/auth/status` that returns `{ "auth_required": true }` when `ORION_ADMIN_*` are set, so the frontend can decide to show Login when `auth_required && !token`. Not strictly necessary; 401-handling is enough.

**API layer**

- [frontend/src/lib/api.ts](frontend/src/lib/api.ts):
  - Read token from `localStorage` (e.g. key `"orion_token"`).
  - For every `request(...)`, if token exists, set `Authorization: Bearer <token>`.
  - If `res.status === 401`: clear `localStorage` token, `window.location.href = "/login"` (or `navigate("/login")` via a passed callback or global router), then throw.
  - Keep `base` as `/v1` (or `VITE_API_BASE_URL`). No change for Docker when served from same origin.

---

## 4. Docker: Dockerfile and Optional docker-compose

**Dockerfile**

- **Location:** `core/Dockerfile` (build context = repo root: `docker build -f core/Dockerfile .`).

**Stages**

1. **Frontend**
   - `FROM node:22-alpine` (or LTS you prefer).
   - `WORKDIR /app`
   - `COPY frontend/package*.json frontend/`
   - `COPY sdk/ sdk/` (frontend depends on `../sdk` for `@orion/sdk`).
   - `RUN cd frontend && npm ci`
   - `COPY frontend/ frontend/`
   - `RUN cd frontend && npm run build`
   - Result: `frontend/dist/`.

2. **Core (Go + assets)**
   - `FROM golang:1.25-alpine` (or the go version in [core/go.mod](core/go.mod)).
   - `WORKDIR /app`
   - `COPY core/go.mod core/go.sum core/`
   - `RUN cd core && go mod download`
   - `COPY core/ core/`
   - `COPY sdk/ sdk/` (only if needed by core; currently core does not import sdk).
   - `COPY --from=0 /app/frontend/dist core/web`
   - `RUN cd core && go build -o orion-core .`
   - Result: `core/orion-core` and `core/web/`.

3. **Runtime**
   - `FROM alpine:3.20` (or `debian-slim` if you prefer).
   - `RUN apk add --no-cache ca-certificates` (for HTTPS if needed later).
   - `WORKDIR /app`
   - `COPY --from=1 /app/core/orion-core .`
   - `COPY --from=1 /app/core/web ./web`
   - `RUN mkdir -p /data`
   - `ENV ORION_DATA_DIR=/data`
   - `ENV ORION_PORT=8999`
   - `EXPOSE 8999`
   - `VOLUME /data`
   - `CMD ["./orion-core"]`

- The binary expects `./web` and `./data` (or `ORION_DATA_DIR`). We set `ORION_DATA_DIR=/data` and `VOLUME /data` so deployers can mount a volume for SQLite.

**Environment variables (document in Docker)**

- `ORION_DATA_DIR` — default `/data` in image.
- `ORION_PORT` — default `8999`.
- `ORION_ADMIN_USERNAME`, `ORION_ADMIN_PASSWORD` — both set to enable frontend login.
- `ORION_JWT_SECRET` — required when `ORION_ADMIN_*` are set.

**docker-compose (optional)**

- **File:** `core/docker-compose.yml` or `docker-compose.yml` at repo root.
- Service: `orion-core`
  - `build: context: . ; dockerfile: core/Dockerfile`
  - `ports: "8999:8999"`
  - `volumes: orion-data:/data`
  - `environment:`
    - `ORION_DATA_DIR: /data`
    - `ORION_PORT: 8999`
    - `ORION_ADMIN_USERNAME: ${ORION_ADMIN_USERNAME:-admin}`
    - `ORION_ADMIN_PASSWORD: ${ORION_ADMIN_PASSWORD:?ORION_ADMIN_PASSWORD is required when using docker-compose}`
    - `ORION_JWT_SECRET: ${ORION_JWT_SECRET:?ORION_JWT_SECRET is required}`
- Use `.env` or `env_file` for secrets in real deployments; the `:?` forces the user to set them when auth is desired.

---

## 5. Makefile and Docs

- **Makefile**
  - `docker-build`: `docker build -f core/Dockerfile -t orion-core:latest .`
  - `docker-up` (optional): `docker compose -f core/docker-compose.yml up -d` (or from root if compose at root).

- **core/README.md** (or main README)
  - How to build: `make docker-build` or `docker build -f core/Dockerfile -t orion-core .`
  - How to run:
    - Without auth: `docker run -p 8999:8999 -v orion-data:/data orion-core`
    - With auth: set `ORION_ADMIN_USERNAME`, `ORION_ADMIN_PASSWORD`, `ORION_JWT_SECRET`.
  - Env reference: `ORION_DATA_DIR`, `ORION_PORT`, `ORION_ADMIN_*`, `ORION_JWT_SECRET`.

---

## 6. Data Flow (High Level)

```mermaid
flowchart TB
  subgraph Docker
    subgraph RuntimeImage
      Binary[orion-core]
      Web[/app/web from frontend dist]
      Data[/data volume]
    end
  end

  subgraph Requests
    Browser -->|"/" or "/agents/..."| Binary
    Binary -->|SPA| Web
    Binary -->|/v1/*| API
    API -->|read/write| Data
  end

  subgraph AuthFlow
    User -->|username, password| Login
    Login -->|JWT| Browser
    Browser -->|Bearer JWT on /v1/*| API
  end
```

---

## 7. Files to Add or Touch

| Action | Path |

| ------ | --------------------------------------------------------------------------------------------------------- |

| Add | `core/internal/config/config.go` |

| Edit | `core/main.go` (config, `dataDir`, `port`, pass auth config into API) |

| Edit | `core/internal/db/db.go` (`Initialize` accepts `dataDir`) |

| Add | `core/internal/api/auth_frontend.go` (login handler, `FrontendAuthMiddleware`) |

| Edit | `core/internal/api/routes.go` (mount `/v1/auth/login`, wrap frontend group with `FrontendAuthMiddleware`) |

| Edit | `core/go.mod` (jwt lib) |

| Add | `frontend/src/pages/LoginPage.tsx` |

| Edit | `frontend/src/App.tsx` (route `/login`, 401 handling / guard as above) |

| Edit | `frontend/src/lib/api.ts` (Bearer token, 401 → clear token and redirect to `/login`) |

| Add | `core/Dockerfile` |

| Add | `core/docker-compose.yml` or `docker-compose.yml` |

| Edit | `Makefile` (`docker-build`, optional `docker-up`) |

| Edit | `core/README.md` or `README.md` (Docker and env) |

---

## 8. Behaviour Summary

- **Docker:** One image contains the built SPA and the Go binary; `ORION_DATA_DIR` and `ORION_PORT` make it configurable; `VOLUME /data` for persistence.
- **Frontend auth:** Enabled only when `ORION_ADMIN_USERNAME` and `ORION_ADMIN_PASSWORD` are both set; `ORION_JWT_SECRET` required in that case. Login form in the SPA; JWT in `Authorization`; 401 on protected `/v1/*` clears token and sends user to `/login`.
- **Non-Docker / dev:** Unset admin env → frontend auth off; `ORION_DATA_DIR` default `data`, `ORION_PORT` default `8999`.
- **Agent auth:** Unchanged; agent routes stay behind `AuthMiddleware` + `ValidateAgentToken`.
