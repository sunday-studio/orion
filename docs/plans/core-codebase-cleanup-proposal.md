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

## Proposed Execution Phases

### Phase 0: Guardrails

Deliverables:

- add this plan;
- create cleanup tickets with file ownership;
- define CI commands to run for every cleanup PR.

Exit criteria:

- Core cleanup work has a visible sequence and does not compete with active feature delivery.

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

2. Split status page handlers by responsibility
   - Description: Move status page admin, component, incident, validation, projection, metadata,
     and helper code into separate files under `internal/api`.
   - Acceptance: Status page tests pass, OpenAPI generation is unchanged except for deterministic
     formatting, and public payload tests still prove internal data is redacted.

3. Extract Core monitor config validation
   - Description: Move kind normalization, per-kind validation, and redaction helpers into a focused
     internal package.
   - Acceptance: Existing Core monitor lifecycle, target-policy, and catalog validation tests pass;
     API response shape does not change.

4. Add Core modernization tooling
   - Description: Add `golangci-lint` v2.6+ modernize and `govulncheck` checks for Core.
   - Acceptance: CI or make targets run both tools, current failures are either fixed or recorded
     as explicit tracked follow-ups.

5. Apply first Core modernization batch
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
- Did modernization reduce code or clarify intent?
- Did modernization avoid experimental APIs and generated-file churn?
- Can the PR be reverted without blocking unrelated feature work?

## Recommendation

Start with Phase 1. Splitting the test surface is the safest first cleanup because it improves
review speed immediately and creates lower-conflict lanes for later handler and service cleanup.
Do not start with a Go version bump or package reshuffle. Those are useful only after the test and
handler surfaces stop hiding unrelated changes.
