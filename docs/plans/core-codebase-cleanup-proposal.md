# Core Codebase Cleanup Proposal

Date: 2026-05-29

## Weakest Point

"Clean up Core" is too broad to execute safely as one refactor. Core now contains API routes,
SQLite models and migrations, services, worker runners, generated Swagger docs, seed data, and
integration tests. A cleanup that moves all of that at once will make review slower and increase the
chance of hiding behavior changes inside file movement.

The cleanup should therefore be a sequence of small vertical slices. Each slice should make one
part of Core easier to read, keep public contracts stable unless explicitly changed, and finish with
focused tests.

## Current Evidence

- `apps/core/go.mod` targets `go 1.25.3`.
- Local audit toolchain was `go1.25.5 darwin/arm64`.
- Go release history lists Go 1.26.3 as the current Go 1.26 patch release on 2026-05-29.
- `golangci-lint` and `govulncheck` were not available locally during the audit.
- `apps/core` has about 54k lines of Go, including generated Swagger docs.
- The largest Core files are generated Swagger docs, `internal/api/integration_test.go`,
  `internal/api/status_pages.go`, `internal/api/alerts.go`, seed data, and several service files.
- Deprecated-package scan found no Core usage of the high-risk packages listed by the
  `golang-modernize` skill, such as `math/rand`, `crypto/elliptic`, `reflect.SliceHeader`,
  deprecated `x/crypto` hash/KDF packages, or `httputil.ReverseProxy.Director`.
- The Core code still has many older readability patterns, especially `interface{}` in JSON payload
  handling and tests, `sort.Slice`, and `context.Background()` in tests.

## Goals

1. Make Core easier to navigate by feature area.
2. Reduce large-file and large-test friction before adding more product surface.
3. Preserve API behavior and generated contract discipline.
4. Apply Go modernization where it improves safety or readability.
5. Add tooling so modernization does not depend on manual review memory.
6. Keep speed high by avoiding framework churn and avoiding broad package reshapes.
7. Bring every backend functionality area under an explicit automated test plan.

## Non-Goals

- Do not replace Gin, GORM, SQLite, or the current API shape during cleanup.
- Do not move `apps/core` into multiple Go modules yet.
- Do not rewrite generated Swagger docs by hand.
- Do not migrate to experimental `encoding/json/v2`.
- Do not combine cleanup with new status page, alert, monitor, or incident features.

## Proposed Shape

### 1. Introduce Feature Test Files

Split `internal/api/integration_test.go` into focused files while keeping the same `api` package and
the same test helper functions.

Suggested files:

- `agent_flow_test.go`
- `monitor_flow_test.go`
- `core_monitor_management_test.go`
- `incident_lifecycle_test.go`
- `alert_flow_test.go`
- `settings_lifecycle_test.go`
- `api_test_helpers.go`

Rules:

- Move tests only; do not change assertions unless a test helper makes the intent clearer.
- Keep helper names stable where possible.
- Run `go test ./internal/api` after each move.
- Do not split across packages until the route/service boundary is cleaner.

Why first: it gives immediate review speed and reduces the worst merge-conflict hotspot without
touching runtime behavior.

### 2. Split Status Page API by Responsibility

`internal/api/status_pages.go` mixes admin CRUD, component mapping, publication validation, public
projection helpers, metadata/theme handling, and utility functions. Split it into responsibility
files inside `internal/api` before deciding whether a new package is justified.

Suggested files:

- `status_page_admin.go`
- `status_page_components.go`
- `status_page_incidents.go`
- `status_page_validation.go`
- `status_page_projection.go`
- `status_page_metadata.go`
- `status_page_helpers.go`

Rules:

- Keep route registration unchanged.
- Keep request/response structs near the handlers that use them.
- Move pure projection helpers away from DB-writing handlers.
- Keep public payload boundary tests close to projection code.

Why second: status pages are broad and user-visible, but the first split can be mechanical and
reviewable if it preserves package and names.

### 3. Extract Core Monitor Config Validation

Core monitor management currently carries lifecycle, validation, redaction, target policy, and
persistence concerns in a large service. Pull type-specific validation into a small internal
validation layer.

Suggested package:

- `internal/coremonitor/validation`

Suggested responsibilities:

- normalize monitor kind aliases;
- validate type-specific config;
- redact config and secret refs;
- expose typed validation errors that API handlers can turn into 400 responses;
- keep target-policy enforcement in service or a separate target-policy package until it has more
  call sites.

Rules:

- Do not change the database schema.
- Do not change JSON response shape.
- Keep table-driven tests for every supported monitor kind.
- Keep worker runner config parsing separate from API request validation unless a shared type is
  clearly safe.

Why third: monitor catalog growth is where future complexity will concentrate, and validation is the
piece most likely to become unreadable if it remains embedded in lifecycle code.

### 4. Standardize JSON Payload Helpers

Core has many `map[string]interface{}` payload builders and test decoders. Replace only owned
non-generated code with `any` and add helpers that make intent visible.

Suggested helpers:

- `jsonObject` or explicit local structs for stable payloads;
- `mustJSON` helpers in test files rather than repeated marshal boilerplate;
- small response decode helpers per test area;
- typed payload structs for worker reports when fields are stable.

Rules:

- Prefer structs for stable API/worker payloads.
- Use `map[string]any` for intentionally open metadata.
- Do not churn generated docs.
- Do not change persisted raw JSON shape without an explicit migration/contract ticket.

Why fourth: this improves readability and makes future contract changes easier to review, but it
should happen after the big files are split.

### 5. Modernize Go Tooling

Add modern tooling before attempting broad syntax cleanup.

Minimum tooling:

- Add `golangci-lint` v2.6 or newer in CI with the `modernize` linter enabled.
- Add `govulncheck` for `apps/core`.
- Add explicit CI jobs or make targets for `core-modernize-check` and `core-vulncheck`.
- Keep generated Swagger and SDK checks separate from modernization checks.

Initial linter policy:

- Enable modernize as advisory first.
- Fix high-signal warnings in small PRs.
- Promote modernize warnings to blocking only after the initial backlog is cleared.

Why fifth: manual modernization does not scale, and the codebase is now large enough that drift will
return without CI pressure.

### 6. Apply Go Modernization in Safe Batches

Use the `golang-modernize` priority order, but apply it to Core in this order:

1. Safety and correctness:
   - run `govulncheck`;
   - verify no deprecated crypto or unsafe header APIs;
   - keep `errors.Is` and `errors.As` patterns where error identity matters.
2. Readability:
   - replace owned `interface{}` with `any`;
   - replace simple `sort.Slice` calls with `slices.SortFunc` where it reads better;
   - use `min` and `max` where manual comparisons are obvious;
   - use `maps` and `slices` helpers for cloning, sorting, and membership when they reduce code.
3. Tests:
   - use `t.Context()` in tests that call context-aware APIs;
   - keep `context.Background()` in process startup code where it is the correct root context;
   - add helpers around repeated API request/response boilerplate.
4. Go 1.26 upgrade:
   - upgrade only after Core, Agent, Console, generated contracts, release packaging, and
     Docker-image paths pass;
   - do not require Go 1.26 syntax until release CI and local contributor tooling are ready.

Avoid:

- changing JSON semantics just to use newer APIs;
- adopting iterators where simple loops are clearer;
- changing logging architecture during cleanup;
- using `sync.WaitGroup.Go` unless a file already has a clear WaitGroup pattern and Go 1.25+ is
  guaranteed in every supported build path.

## Backend Functionality Test Plan

The backend should not rely on one giant integration file as proof that everything works. The target
state is a test inventory where each Core behavior has at least one owner, one expected test level,
and one CI gate. "Every functionality" means every route group, service behavior, worker runner,
persistence rule, scheduler, security boundary, and public/private projection path that Core owns.

### Test Taxonomy

- Unit tests: pure validation, normalization, redaction, payload shaping, sorting, and small helper
  behavior.
- Service tests: database-backed service behavior with SQLite test databases and no HTTP router.
- API integration tests: route, auth, request validation, response shape, generated-contract-facing
  behavior, and cross-service flows.
- Worker tests: Core monitor worker claim, execution, result persistence, incident reconciliation,
  and missed-heartbeat behavior.
- Migration tests: schema ordering, forward migration, default values, indexes, and compatibility
  with existing rows.
- Contract tests: OpenAPI generation, generated SDK drift, and response redaction for public and
  frontend routes.
- Security tests: auth boundaries, token lifecycle, target policy, secret redaction, public status
  data isolation, rate limits, and unsafe-input rejection.
- Operational tests: diagnostics, lifecycle scheduler, archive, rollup, worker heartbeat, service
  logs, and backup-sensitive persistence paths.

### Coverage Matrix

| Area | Minimum coverage | Current direction |
| --- | --- | --- |
| Auth and frontend sessions | API integration plus service tests | Login, configured auth, JWT, expired/missing token, and rate-limit behavior should be split out of the monolithic integration test. |
| Agent registration and token lifecycle | API integration plus service tests | Cover register, report auth, rotate, revoke, reissue, revoked-token rejection, and machine identity preservation. |
| Agent report ingestion | API integration plus service tests | Cover system reports, config summary redaction, stale monitor reconciliation, diagnostics recording, and bad payload rejection. |
| Monitor registration and reports | API integration plus service tests | Cover register, unregister, report ownership, stale state, computed health cache invalidation, and incident opening/resolution. |
| Core monitor management | API integration plus service tests | Cover create, edit, pause, resume, delete, test-now, redaction, kind aliases, validation, target policy, and unsupported kinds. |
| Core monitor scheduler | Service tests plus worker tests | Cover lease claiming, lease expiry, completion, next-run scheduling, paused monitors, and multiple-worker safety. |
| Core worker runners | Worker tests per runner | Keep one focused file per runner covering success, failure, invalid config, timeout, payload shape, redaction, and target-policy behavior. |
| Heartbeats | API integration plus worker tests | Cover token generation, success/failure ingest, payload truncation/redaction, pending state, missed-check reconciliation, recovery, and incident behavior. |
| Incident lifecycle | API integration plus service tests | Cover open, acknowledge, resolve, cover, reopen, unregister cleanup, event timeline, filters, candidates, recurrence, and public-impact fields. |
| Alerts | Service tests plus API integration tests | Cover webhook destinations, editable rules, dry runs, grouping, cooldowns, maintenance suppression, test actions, delivery attempts, outbound target policy, and secret redaction. |
| Status pages admin | API integration tests | Cover page CRUD, sections, components, mappings, incidents, updates, publish validation, audit events, metadata, custom domains, and theme settings. |
| Status pages public | API integration plus contract tests | Cover public payloads, HTML, Atom feed, badges, history, uptime, cache headers, ETags, custom-domain routing, and internal data isolation. |
| Public subscribers | API integration plus service tests | Cover subscribe, confirm, preferences, unsubscribe, fan-out scoping, public mail sender config, token secrecy, and abuse controls. |
| Settings and data lifecycle | Service tests plus API integration tests | Cover read/update settings, manual archive, manual rollup, scheduled archive/rollup, last-run metadata, disabled jobs, and failure visibility. |
| Runtime diagnostics | Service tests plus API integration tests | Cover request counts, ingestion latency, report writes, DB stats, slow operations, SQLite busy counts, and frontend auth boundary. |
| Worker diagnostics | Service tests plus API integration tests | Cover heartbeat recording, stale worker status, API health separation, and worker status response shape. |
| Service logs | API integration plus service tests | Cover batch ingest, dedupe/fingerprint, filters, levels, truncation, redaction, source separation, and pagination. |
| Events and audit logs | API integration plus service tests | Cover event creation, source/type/search filters, pagination, audit actor fields, and sensitive-field minimization. |
| Persistence and migrations | Migration tests | Cover migration ordering, status page migration ordering, default values, index assumptions, and old-row compatibility for changed schemas. |
| Config and startup | Unit plus startup tests | Cover env parsing, defaults, required auth settings, CORS, mail config, target policy flags, data paths, and invalid config errors. |
| Generated contracts | Contract tests | Cover OpenAPI generation, generated SDK drift, route annotations for changed API handlers, and no hand-edited generated outputs. |

### Coverage Execution Plan

1. Inventory before adding tests:
   - generate a route list from `setupRoutes`;
   - list service public methods;
   - list worker runner kinds;
   - list migrations and operational jobs;
   - map each item to an existing test or mark it as missing.
2. Split before expanding:
   - move existing tests into feature files first;
   - add `api_test_helpers.go` so new tests are shorter and less brittle;
   - avoid adding more scenarios to `integration_test.go`.
3. Fill high-risk gaps first:
   - auth and token lifecycle;
   - Core monitor target policy and secret redaction;
   - public status page privacy boundary;
   - alert delivery side effects;
   - migrations and data lifecycle jobs.
4. Add functionality gates:
   - every new Core route must include API integration coverage;
   - every new service method with branching logic must include service tests;
   - every new worker runner must include success, failure, invalid config, timeout, and redaction tests;
   - every migration must include a migration or compatibility test when it changes defaults,
     indexes, or existing rows.
5. Add coverage reporting without chasing vanity metrics:
   - start with package-level coverage visibility in CI;
   - require explicit review notes for untested backend behavior;
   - later set package thresholds only after generated docs and integration-heavy packages are
     excluded or interpreted correctly.

### Backend Test CI Gates

Minimum CI commands:

```sh
cd apps/core && go test ./...
cd apps/core && go test -race ./internal/service ./internal/worker
make generate-openapi
git diff --exit-code -- apps/core/docs apps/core/openapi.yaml apps/console/src/orion-sdk
```

Optional but recommended once tooling is installed:

```sh
cd apps/core && govulncheck ./...
cd apps/core && golangci-lint run
```

The race detector should start on service and worker packages because they carry scheduling,
leasing, diagnostics, and background-job behavior. Running `-race ./...` can be added later if CI
time stays acceptable.

### Backend Test Completion Definition

A backend area counts as covered only when:

- the happy path is tested;
- at least one invalid input path is tested;
- auth or ownership boundaries are tested when the area exposes HTTP routes;
- secret redaction is tested when payloads can include credentials, tokens, headers, URLs, or raw
  output;
- persistence side effects are asserted in the database when the behavior writes state;
- public routes prove they do not expose internal-only fields;
- generated contract drift is checked when route annotations or response shapes move.

This is intentionally stricter than line coverage. A high line-coverage number is not enough if the
security boundary, persistence side effect, or public/private projection is untested.

## Proposed Execution Phases

### Phase 0: Guardrails

Deliverables:

- add this plan;
- create cleanup tickets with file ownership;
- define CI commands to run for every cleanup PR.
- create the backend functionality test inventory.

Exit criteria:

- Core cleanup work has a visible sequence and does not compete with active feature delivery.
- Every Core backend area has an owner row in the testing matrix.

### Phase 1: Test Surface Split

Deliverables:

- split `internal/api/integration_test.go`;
- extract shared test helpers;
- keep all tests in package `api`.

Verification:

```sh
cd apps/core && go test ./internal/api
cd apps/core && go test ./...
```

Exit criteria:

- no single API test file owns unrelated incident, monitor, alert, status page, and settings flows.

### Phase 1b: Backend Test Inventory And Gaps

Deliverables:

- create a route-to-test inventory for Core API routes;
- create a service-method-to-test inventory for Core services;
- create a runner-kind-to-test inventory for Core worker runners;
- create a migration-to-test inventory for persistence changes;
- mark each item as covered, partial, missing, or intentionally deferred.

Verification:

```sh
cd apps/core && go test ./...
```

Exit criteria:

- missing backend test coverage is visible as discrete tickets, not hidden inside the cleanup plan.

### Phase 2: Status Page Handler Split

Deliverables:

- split `status_pages.go` into responsibility files;
- keep public/admin/status-page cache/feed/badge files intact unless directly related;
- keep OpenAPI annotations attached to handlers.

Verification:

```sh
cd apps/core && go test ./internal/api -run StatusPage
cd apps/core && go test ./...
make generate-openapi
git diff --exit-code -- apps/core/docs apps/core/openapi.yaml
```

Exit criteria:

- status page CRUD, public projection, validation, and incident publishing can be reviewed
  separately.

### Phase 3: Core Monitor Validation Package

Deliverables:

- move kind normalization and config validation into a focused internal package;
- keep lifecycle persistence in the service;
- keep API response shape stable.

Verification:

```sh
cd apps/core && go test ./internal/service -run CoreMonitor
cd apps/core && go test ./internal/api -run CoreMonitor
cd apps/core && go test ./internal/worker
```

Exit criteria:

- adding a monitor kind requires touching a predictable validator, runner, tests, and optional UI
  code instead of searching through one large service.

### Phase 4: Modernization Tooling

Deliverables:

- add `golangci-lint` v2.6+ config with modernize enabled;
- add `govulncheck` to CI or documented make target;
- document ignored suggestions in `.modernize` only when there is an explicit decision.

Verification:

```sh
cd apps/core && go test ./...
cd apps/core && golangci-lint run
cd apps/core && govulncheck ./...
```

Exit criteria:

- future Core cleanup receives automated modernization feedback.

### Phase 5: Modernization Batches

Deliverables:

- replace obvious `interface{}` with `any` in non-generated Core files;
- modernize simple sort and collection helpers;
- switch context-aware tests to `t.Context()` where it improves cancellation behavior;
- consider Go 1.26 module upgrade only after CI tooling and release packaging are proven.

Verification:

```sh
cd apps/core && go test ./...
make generate-openapi
cd apps/console && pnpm run build
```

Exit criteria:

- code is easier to read without behavior changes or generated contract drift.

## First Tickets To Create

1. Split Core API integration tests
   - Description: Move unrelated tests out of `internal/api/integration_test.go` into feature files
     and extract shared request helpers.
   - Acceptance: `go test ./internal/api` and `go test ./...` pass, and no handler/service behavior
     changes are included.

2. Inventory backend functionality test coverage
   - Description: Build a route, service, worker, migration, diagnostics, and operational-job
     coverage inventory for Core.
   - Acceptance: Every Core backend area is marked covered, partial, missing, or intentionally
     deferred, and missing/partial rows have follow-up tickets with acceptance criteria.

3. Add high-risk backend coverage gaps
   - Description: Add tests for any missing auth, token lifecycle, target-policy, public/private
     status page, alert side-effect, migration, or data lifecycle behavior found by the inventory.
   - Acceptance: High-risk missing rows are covered by focused service, API integration, worker, or
     migration tests, and `go test ./...` passes in `apps/core`.

4. Split status page handlers by responsibility
   - Description: Move status page admin, component, incident, validation, projection, metadata,
     and helper code into separate files under `internal/api`.
   - Acceptance: Status page tests pass, OpenAPI generation is unchanged except for deterministic
     formatting, and public payload tests still prove internal data is redacted.

5. Extract Core monitor config validation
   - Description: Move kind normalization, per-kind validation, and redaction helpers into a focused
     internal package.
   - Acceptance: Existing Core monitor lifecycle, target-policy, and catalog validation tests pass;
     API response shape does not change.

6. Add Core modernization tooling
   - Description: Add `golangci-lint` v2.6+ modernize and `govulncheck` checks for Core.
   - Acceptance: CI or make targets run both tools, current failures are either fixed or recorded
     as explicit tracked follow-ups.

7. Apply first Core modernization batch
   - Description: Replace obvious `interface{}` with `any`, modernize simple `sort.Slice` usage,
     and convert eligible tests to `t.Context()`.
   - Acceptance: `go test ./...` passes in `apps/core`, generated files are untouched unless
     regeneration is explicitly required, and diffs are mechanical/readability-only.

## Review Checklist For Each Cleanup PR

- Does the PR change behavior, or only move/rename/readability code?
- If behavior changed, is there an explicit product ticket?
- Are OpenAPI annotations still attached to route handlers?
- Did generated files change only through the approved generation commands?
- Are tests split by feature rather than by incidental helper ownership?
- Does each backend behavior touched by the PR have unit, service, API, worker, migration, or
  contract coverage at the right level?
- Are missing backend tests converted into tracked follow-up tickets before merge?
- Did modernization reduce code or clarify intent?
- Did modernization avoid experimental APIs and generated-file churn?
- Can the PR be reverted without blocking unrelated feature work?

## Recommendation

Start with Phase 1. Splitting the test surface is the safest first cleanup because it improves
review speed immediately and creates lower-conflict lanes for later handler and service cleanup.
Do not start with a Go version bump or package reshuffle. Those are useful only after the test and
handler surfaces stop hiding unrelated changes.
