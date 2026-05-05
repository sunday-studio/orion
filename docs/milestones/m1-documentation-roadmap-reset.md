# M1: Documentation and Roadmap Reset

## Goal

Bring the project roadmap back in sync with the current repository and create a durable place for milestone notes and planning.

## Scope

- Audit current repo structure, docs, API routes, agent CLI, frontend routes, and recent verification results.
- Replace the stale `todo.md` with a current checklist.
- Add milestone and plan directories under `docs/`.
- Create a product hardening plan for the next phase.

## Completed

- Root roadmap now reflects implemented Core, Agent, Frontend, Docker, API, and health-computation work.
- Remaining work is grouped by documentation, Core, Agent, Frontend, testing, CI/CD, and release.
- `docs/milestones/` now exists for milestone records.
- `docs/plans/` now exists for product and engineering plans.

## Verification

- `go test ./...` passed for `core` after dependencies were downloaded.
- `go test ./...` passed for `agent` after dependencies were downloaded.
- Frontend build was not verified because `apps/console/node_modules` is not installed in this checkout.

## Open Risks

- The frontend may have type or build drift until dependencies are installed and `npm run build` is run.
- Go test coverage is still thin; passing tests mostly confirm compilation.
- Older docs still contain some Phase 1 language that may not match the current API and product behavior.

## Next

- Establish a verification baseline in CI.
- Add API/service tests around registration, monitor lifecycle, report storage, and health computation.
- Install frontend dependencies and validate build/typecheck.
