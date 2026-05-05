# M2: Directory Regrouping for Agent Work

## Goal

Group the repository by clear ownership boundaries so humans and coding agents can work in parallel without guessing which copy of a component is authoritative.

## Scope

- Move deployable apps under `apps/`.
- Move generated/shared SDK material under `packages/`.
- Move runtime deployment assets under `deploy/`.
- Keep docs, plans, and milestones under `docs/`.
- Add root `AGENTS.md` with ownership and contract rules.

## Completed

- `agent/` moved to `apps/agent/`.
- `core/` moved to `apps/core/`.
- `frontend/` moved to `apps/web/`.
- Removed legacy/prototype `core/console/` so `apps/web/` is the only product UI.
- `sdk/` moved to `packages/sdk/`.
- `docker-compose.yml` moved to `deploy/docker-compose.yml`.
- Service and install helper scripts moved under `deploy/`.
- Makefile, Dockerfile, Docker Compose, Orval config, README, and roadmap paths were updated.
- `.gitignore` now covers grouped local data, dependencies, and frontend build outputs.

## Verification

- `cd apps/core && go test ./...` passed.
- `cd apps/agent && go test ./...` passed.
- `make -n generate-sdk build-static docker-build docker-up` expands to the new grouped paths.
- `cd apps/web && npm run build` still fails until dependencies are installed because `tsc` is missing.

## Open Risks

- Frontend build remains unverified after the move until `apps/web/node_modules` is installed.
- Git will show many deletes and adds from the directory move until the changes are staged and rename detection can be reviewed.
- `apps/web/` is now the only product UI; future frontend work should not create a second app without an explicit product decision.

## Next

- Install `apps/web` dependencies and run the web build.
- Run `make build-static` once the web build is healthy.
- Consider adding CI path filters around `apps/agent`, `apps/core`, `apps/web`, and `deploy`.
