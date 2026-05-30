# Settings PR Closeout Checklist

Use this checklist for Settings readiness PRs that touch lifecycle settings,
manual maintenance actions, lifecycle auditability, or Console Settings UX.

## Required Evidence

- Tests: list the exact Core, Console, and browser smoke commands that passed. If a command was not run, state why and name the remaining owner.
- Generated contracts: confirm whether Core route annotations, `apps/core/openapi.yaml`, Swagger docs, or `apps/console/src/orion-sdk/index.ts` changed. Regenerate them when API behavior or response shape changes.
- Docs: update `docs/architecture/persistence-and-lifecycle.md` when archive, rollup, scheduler, audit, or Settings API behavior changes.
- Maat: comment on every ticket touched by the PR, complete tickets only with test, commit, or PR evidence, and leave explicit follow-up comments for in-flight work.
- PR evidence: include the branch, commit, PR URL, relevant tests, generated-file status, and known blocked checks in the PR body or closing comment.

## Settings-Specific Gates

- Archive directory behavior is documented and tested before claiming path-safety readiness.
- Manual archive behavior is verified for enabled, disabled, zero-report, and failed-path states before claiming operator safety readiness.
- Manual rollup behavior is verified for default yesterday runs and explicit date runs before claiming lifecycle correctness.
- Lifecycle events are visible in Logs before claiming audit visibility, and durable `audit_events` rows exist before claiming auditability.
- Console Settings browser coverage reaches read, save, invalid form, manual rollup, and manual archive paths before claiming E2E trust.
