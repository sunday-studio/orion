# Settings Readiness Closeout

Use this checklist for Settings readiness PRs and Maat ticket closeout.

## Scope

The first-release Settings surface is data lifecycle operations:

- read and update `data_lifecycle_settings`;
- run manual monitor uptime rollups;
- run manual raw report archives;
- show last rollup and archive result state;
- expose lifecycle activity in operational logs.

Server preferences, user profile settings, alert destination settings, and status page settings are separate product surfaces.

## Readiness Goal Map

- `G-20260529-191050-64de`, "Settings readiness: restore E2E trust": browser smoke selectors, read/save coverage, validation coverage, and manual action coverage.
- `G-20260529-191055-6c42`, "Settings readiness: harden archive path security": archive directory policy, validation, and security tests.
- `G-20260529-191101-d8bd`, "Settings readiness: make manual maintenance safe": archive confirmation, result states, and duplicate action prevention.
- `G-20260529-191107-307a`, "Settings readiness: add lifecycle auditability": settings update audit events, lifecycle action audit events, and Logs visibility.
- `G-20260529-191112-26c9`, "Settings readiness: improve lifecycle product UX": operator sections, inline validation, and recent activity context.
- `G-20260529-191118-066d`, "Settings readiness: close planning and docs": architecture docs, Maat reconciliation, and this checklist.

The duplicate audit tickets `T-20260529-182154-8388` and `T-20260529-182206-4e9a` should resolve to the same evidence: the Settings readiness plan led to the six follow-up goals above. Do not create more title-only Settings tickets; attach new work to one of the readiness goals or add a factual Maat comment explaining why a new goal is needed.

## PR Checklist

- Code changes stay in the correct boundary: Core API/service work in `apps/core/`, Console work in `apps/console/`, docs in `docs/`, generated Core static assets only from `make build-static`.
- API shape changes update Core route annotations, `apps/core/openapi.yaml`, and `apps/console/src/orion-sdk/index.ts`.
- Settings behavior changes update `docs/architecture/persistence-and-lifecycle.md` when they affect archive path policy, scheduler behavior, audit events, manual action semantics, or uptime read behavior.
- Core validation or lifecycle service changes run `go test ./internal/service ./internal/api -run 'Test(DataLifecycle|SettingsService)'` from `apps/core`; broader Core changes run `go test ./...`.
- Console Settings changes run the Console build and the relevant browser smoke or Playwright Settings coverage.
- Manual archive PRs verify confirmation copy, disabled/loading states, result states, and duplicate-click prevention.
- Auditability PRs verify that sensitive values are not stored in audit metadata or rendered in Logs.
- Every Settings readiness ticket has a Maat comment or completion evidence naming tests, docs, generated files, branch, commit, or PR.
- PR descriptions list generated files separately from hand-edited files.

## Closeout Evidence

At closeout, record:

- branch and PR URL;
- changed files;
- tests or checks run;
- generated files changed or explicitly not needed;
- Maat tickets completed and evidence used;
- known follow-up tickets that remain open.
