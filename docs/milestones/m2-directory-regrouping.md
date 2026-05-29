# M2: Directory Regrouping for Server Work

## Goal

Group the repository by clear ownership boundaries.

## Scope

- Move deployable apps under `apps/`.
- Move generated/shared SDK material under `packages/`.
- Move runtime deployment assets under `deploy/`.
- Keep docs, plans, and milestones under `docs/`.
- Add root `AGENTS.md` with ownership and contract rules.

## Completed

- `agent/` moved to `apps/agent/`.
- `core/` moved to `apps/core/`.
- `frontend/` moved to `apps/console/`.
- Removed legacy/prototype `core/console/` so `apps/console/` is the only product UI.
- `sdk/` moved to `packages/sdk/`.
- `docker-compose.yml` moved to `deploy/docker-compose.yml`.
- Service and install helper scripts moved under `deploy/`.
- Makefile, Dockerfile, Docker Compose, Orval config, README, and roadmap paths were updated.
- `.gitignore` now covers grouped local data, dependencies, and frontend build outputs.

## Verification

- `cd apps/core && go test ./...` passed.
- `cd apps/agent && go test ./...` passed.
- `make -n generate-sdk build-static docker-build docker-up` expands to the new grouped paths.
- `cd apps/console && npm run build` still fails until dependencies are installed because `tsc` is missing.

## Open Risks

- Frontend build remains unverified after the move until `apps/console/node_modules` is installed.
- Git will show many deletes and adds from the directory move until rename detection can be reviewed.
- `apps/console/` is now the only product UI.

## Next

- Install `apps/console` dependencies and run the console build.
- Run `make build-static` once the web build is healthy.
- Consider adding CI path filters around `apps/agent`, `apps/core`, `apps/console`, and `deploy`.
