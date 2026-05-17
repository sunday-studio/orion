# M1: Documentation and Roadmap Reset

## Goal

Bring the project roadmap back in sync with the current repository.

## Scope

- Audit current repo structure, docs, API routes, agent CLI, frontend routes, and recent verification results.
- Retire the stale root roadmap and move planning history into milestone records.
- Add milestone records under `docs/`.
- Create a product hardening plan for the next phase.

## Completed

- Root roadmap now reflects implemented Core, Agent, Frontend, Docker, API, and health-computation work.
- Remaining work is grouped by documentation, Core, Agent, Frontend, testing, CI/CD, and release.
- `docs/milestones/` now exists for milestone records.
- Product and engineering planning history is now consolidated into milestone records.

## Verification

- `go test ./...` passed for `core` after dependencies were downloaded.
- `go test ./...` passed for `agent` after dependencies were downloaded.
- Frontend build was not verified because `apps/console/node_modules` is not installed in this checkout.

## Open Risks

- The frontend may have type or build drift until dependencies are installed and `npm run build` is run.
- Go test coverage is still thin; passing tests mostly confirm compilation.

## Next

- Establish a verification baseline in CI.
- Add API/service tests around registration, monitor lifecycle, report storage, and health computation.
- Install frontend dependencies and validate build/typecheck.
