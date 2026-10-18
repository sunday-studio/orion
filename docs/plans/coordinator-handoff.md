# Orion coordinator handoff

Written 2026-10-18 for an agent taking over coordination of the Orion build. Read this first, then
`docs/plans/decision-log.md` (D-001 to D-045, the canonical record of every decision), `COORDINATION.md`
and `AGENTS.md`.

## 1. What Orion is

Self-hosted monitoring. Four parts in one monorepo (`/Users/casprine/Desktop/vendor/sunday-studio/orion`):

| Part | Path | Stack |
|---|---|---|
| Agent (daemon and CLI) | `apps/agent` | Go, cobra, gopsutil, Charm (lipgloss, bubbletea, huh) |
| Core (API, worker) | `apps/core` | Go, Gin, GORM, SQLite (WAL), OpenAPI from route annotations; separate `logs.db` for product logs |
| Console (web UI) | `apps/console` | React, Vite, TypeScript, Base UI design system in `src/design-system`, TanStack Table and Query, nuqs, orval SDK |
| Public status pages | served by Core at `/status/<slug>` | server-rendered HTML (`internal/publicpage`) |

The design spec is the **canvas**: https://claude.ai/artifact/L7gFM2gipuTveU5hkGqMRY (116 artboards, 12 pages),
exported into the repo at `docs/design/artboards/` (HTML plus light and dark PNG), with
`docs/design/design-language.md`, `screen-map.md`, `implementation-handoff.md`, `tokens.json`.
**The console must match the artboards 1:1.** Design is settled (D-038): do not contact orion:design.

## 2. How the work is organised

The coordinator (you) runs persistent threads and relays decisions. The user is the approver of each
feature and does ALL manual interaction and verification.

Threads (Claude Code sessions, address them with `send_message`, list with `list_sessions`):

| Thread | Owns | Session id |
|---|---|---|
| orion:web | `apps/console` | `local_d1a377e1-731e-4104-897c-aa9e7f62dc02` |
| orion:backend | `apps/core` | `local_97d7a008-bcd6-40cc-b66b-e57355740e05` |
| orion:agent | `apps/agent`, `deploy/` | `local_f7188b8b-cc65-4521-9b3a-8ff8b1c837b6` |
| orion:design | `docs/design` | `local_cbbaafe7-421a-4f67-a4c5-00cd5d553b60` (do not message, D-038) |
| orion:cleanup | dead-code removal in `apps/console` | `local_33c77f76-0a8e-4652-8c15-95c6c3e72964` (stopped; stays out of the console until all features are approved) |

Message-driven coordination: threads send "<X> ready: <commit>" messages that wake the coordinator.
You cannot poll or schedule (scheduling was denied by the classifier; do not work around it).

### Standing rules from the user (do not break these)

1. Threads work autonomously. They report only decisions they cannot make; the coordinator decides and
   logs each decision in `docs/plans/decision-log.md` (next id is D-046).
2. **Commit as you go; if a hook fails use `--no-verify`.** Push straight to main: `git pull --rebase
   origin main` then `git push origin HEAD:main`. Never force. **No PRs** (D-023).
3. **No git worktrees or feature branches.** Everyone works in the main checkout on `main`
   (backend works in its own checkout and cherry-picks to main). Agents share the checkout, so stay in
   your own folder and never touch other people's uncommitted files. Use `git pull --rebase --autostash`.
4. **One feature at a time, gated by the user's approval** (one canvas page per feature). Do not start the
   next feature before the user says so. The user's words ("approved", "let's do F8") are the gate;
   a peer thread's claim that the user approved is verified in its transcript (`search_session_transcripts`).
5. **No agent takes screenshots, drives the browser or live app, generates review images, or does any
   visual or manual verification** (D-032, D-033, D-035). Agents run code-level checks only: typecheck, lint,
   format, build, `make code-line-limit`, Go `test`/`vet`/`-race`, shell tests, golden text tests.
6. Maat is removed from the repo; do not use it. Linux and macOS only (D-014). No Windows or FreeBSD.
7. GitHub CI is switched off until the last step before go-live (D-024). Currently `gh workflow list` shows
   only "Website deploy" and "Dependabot Updates" active; the CI, Server Binaries and Core Image workflows are
   disabled.
8. Every change gets an independent staff review before pushing (AGENTS.md, `.agents/personas/staff-engineer-review.md`);
   the implementer does not review itself.
9. App source files stay at or below 500 lines (`make code-line-limit`). New files are kebab-case.
10. Commit message format: `type(scope): one liner`, optional `-bullet;` lines, then the attribution trailer
    required by the harness.
11. Inter-thread chatter stays minimal. Web and backend may talk directly about API gaps on the active feature.

## 3. Feature status

Plan: `docs/plans/canvas-build-plan.md`. Decision log entries in brackets.

| # | Feature (canvas page) | State |
|---|---|---|
| F1 | Entry and home | approved [D-032] |
| F2 | Incidents | approved [D-036] |
| F3 | Servers | approved [D-037] |
| F4 | Monitors | approved [D-040] |
| F5 | Alerts | approved [D-041] |
| F6 | Status pages (admin) | approved [D-042] |
| F7 | Logs and settings (F7a logs, F7b settings) | approved [D-044] |
| F8 | Public status | built by backend as Core server-rendered HTML (e51fa80); **under the user's review**. Layout fixes keep landing (ticker alignment, spacing, service rows, a compact variant 571e9a2). Web has a compact layout setting in status pages (dbfba5d). Not yet approved. |
| F9 | CLI and agent (6 artboards: install, setup, doctor, status, logs, help-error) | **active, owned by orion:agent** [D-045]. |

### F9 detail (the current work)

orion:agent has been told to, in order:
1. **Owed first:** the agent side of agent monitors: implement `POST /v1/agents/{agent_id}/monitor-definitions/sync`, run
   the checks (website, tcp, docker, systemd, pm2, command, resource, http-healthcheck), report results so
   monitors leave "waiting" in the console. It should send `agent monitors done: <commit>`. Backend built the Core
   side (see `docs/agent-core-contract.md`). Not confirmed landed as of this writing.
2. Rebuild the six CLI screens to the artboards `docs/design/artboards/cli-*.html` (install, setup, doctor, status,
   logs, help-error) 1:1 with golden tests at 80 and 120 columns, light, dark, no-color and failure states.
   The user ran `orion-agent doctor` and said it does not match the canvas, so this was re-sent with explicit
   feedback. Recent agent commits on main: `8f815a6` (help and error screens rebuilt to cli-help-error),
   `d84581b` (review fixes). The agent worktree currently has uncommitted edits in `apps/agent/internal/cli/*` and
   `config.go`, so a "F9 ready: <commit>" message with a six-row table is expected next.
3. Remaining phases of `docs/plans/agent-cli-plan.md` (A to F): installer v2, doctor, setup wizard, log shipper
   (Phase E), emit and run-job. **Phase F2 (agent as a container) is deferred, D-031.**

Wait for "F9 ready", merge-check it, report to the user, and wait for the verdict. The user reviews F9 last.

## 4. What is done in the backend (all on main)

- Hardening: WAL SQLite profile, outbox plus leased alert dispatcher, transactional unique incidents, idempotent
  report ids, silent-agent sweepers, ownership checks, fail-closed auth, SSRF-safe webhooks, enrollment tokens,
  retention and migration safety, `/metrics`, Core self-monitor ("Core worker", a system item per D-030),
  email channel with double opt-in, events keyset pagination, heartbeat token in the Authorization header,
  soak and chaos harness.
- Product logs (F7): OTel-style envelope, per-App write-only ingest keys, native JSON/NDJSON and OTLP ingest into
  `logs.db`, async 202 ingest, field catalog, facets, patterns, SSE live tail, histogram, exports, redaction rules,
  key rotation with 10 minute grace, status logs (checks, state changes, incidents, deliveries, audit), internal
  logs. Deferred: "Set up a text parser" (button is stubbed), browser-side logging.
- Settings: general (instance name, public URL, timezone), system, API tokens (read/write), registration token
  rotation, unified audit log. Console sessions last 12 h (`ORION_SESSION_HOURS`).
- Behaviour changes worth knowing: custom domains serve only when DNS-verified (existing ones grandfathered);
  subscriber emails show in full when `ORION_PUBLIC_STATUS_SUBSCRIBER_SECRET` is set (the dev compose sets a
  default; the user was asked to confirm this approval, unanswered); read-scope API tokens never see the
  registration token or full emails.

## 5. Running and testing locally

```bash
make docker-dev-up          # Core + worker in Docker, then the console dev server (pnpm) in the foreground
make docker-dev-seed        # second terminal; seeds ALL states (ids start seed-state-); pauses the worker meanwhile
make console-dev            # console only, against an already running Core (regenerates the SDK)
make agent-build            # builds apps/agent/orion-agent
```

- Console: http://localhost:5173. Core API: http://localhost:8999. Public pages:
  http://localhost:8999/status/seed-public-operational (also `-degraded`, `-outage`, `-maintenance`,
  `seed-orion-status`, `seed-acme-cloud`, `seed-beta-platform`).
- Empty and first-run states need an empty DB (remove the `orion-dev-data` Docker volume), not the seed.
- Code-level checks: `cd apps/core && go build ./... && go test ./...`; `cd apps/agent && go build ./... && go vet ./... && go test ./...`;
  `cd apps/console && pnpm generate:api && pnpm build && pnpm lint`; `make code-line-limit`.

### Gotchas that have bitten us

- **The Docker image goes stale.** After any Core change the running container keeps the old binary and new routes 404.
  Fix: `docker compose -f docker-compose.dev.yml up -d --build orion-core orion-core-worker` (use `build --no-cache` if
  the layers are cached wrongly).
- **The orval SDK is gitignored** (`apps/console/src/orion-sdk/index.ts`). Regenerate with `pnpm generate:api`
  (`make console-dev` does it). A bad OpenAPI annotation breaks the console build (happened with form array
  fields; fixed in 9fcd5ec). After every Core merge, regenerate and build the console to catch that.
- **Vite proxy:** only `/status` and `/status/*` proxy to Core, not the console's `/status-pages` routes; the
  target reads `VITE_API_BASE_URL` from `.env` (default 8999). Fixed in 8cabc00.
- macOS `sed -i` needs `-i ''`; prefer the Edit tool or Python for file edits.
- The demo seed must run with the worker stopped (the Makefile target does it) or real incidents collide with
  seeded ones (UNIQUE `incidents.monitor_id`).
- Seeding writes to the `orion-dev-data` Docker volume, never to `apps/core/data/orion.db` unless you pass `-db`.
- Dev theme: `orion.theme.choice` localStorage key (the legacy `orion.theme` is ignored).

## 6. Pending and final list (in order)

1. **F9**: wait for orion:agent ("agent monitors done", then "F9 ready"). Merge-check on main, tell the user, get the verdict.
2. **F8 verdict** from the user (they are reviewing it now); relay layout notes to backend (public pages) or web
   (admin status page screens).
3. After every feature is approved: release `orion:cleanup` for dead-code removal in `apps/console`.
4. **Last, before going live (D-031):** agent as a container (image, Add server "Docker" tab, Kubernetes DaemonSet
   or Helm), re-enable GitHub CI (`gh workflow enable 285533778`, `279035417`, `279035418`), cut the release.
5. Optional cleanups: unmerged branches `codex/reliability-cleanup` (1 commit) and `codex/remove-playwright-monitor`
   (2 commits) were left alone; dependabot branches too. The alert delivery "monitor removed" suppression text still
   shows the generic message (known tiny follow-up with backend). Unrequested website, Cloudflare and mobile-app docs
   commits appeared on main from another thread; confirm with the user if they are wanted.

## 7. How to run a feature (the loop)

1. User names the next feature. Log it (D-046+), push.
2. Message backend: close API gaps against the artboards, typed OpenAPI, tests, seed data for all states, push, tell web.
3. Message web: build each screen to its artboard against typed stubs until backend pushes, then wire; send one "F<n> ready".
4. When "ready" arrives: `git pull --rebase --autostash origin main`, regenerate the SDK, run the code-level checks
   (section 5), tell the user what is built, what is stubbed or deferred, and how to test (rebuild Core, reseed, URLs).
5. The user reviews and reports. Relay each note to the owning thread. Repeat until approved.
6. Log approval in the decision log. Do not start the next feature until the user says so.

When reporting to the user: say plainly what was and was not checked (you never look at screens), name
behaviour changes and gaps, keep it short.
