# First Deploy Todo

This checklist tracks the work needed before Orion is ready for a first self-hosted deploy candidate.

## Priority 0 - Commit Current Work

- [x] Implement interval-based stale detection for agents and monitors.
- [x] Send agent reporting interval during Agent registration.
- [x] Send monitor reporting interval during monitor registration.
- [x] Regenerate OpenAPI and Console SDK from Core annotations.
- [x] Update Agent/Core contract and architecture docs for interval-aware stale rules.
- [x] Commit interval-based stale detection changes.

## Priority 1 - Core Deploy Smoke

- [x] Fix Docker Compose build context for `make docker-up`.
- [x] Add Core runtime serving for Console static assets.
- [x] Add build toolchain dependencies needed for SQLite CGO in Docker build.
- [x] Add restart policy and `/health` container healthcheck to Docker Compose.
- [x] Make Docker Compose require frontend auth environment variables.
- [x] Implement `make build-static`.
- [x] Build the Core Docker image with `make docker-build`.
- [x] Start Core with Docker Compose using `make docker-up`.
- [x] Confirm `/health` returns healthy.
- [x] Confirm the Console loads from the Core container.
- [x] Confirm login works with `ORION_ADMIN_USERNAME`, `ORION_ADMIN_PASSWORD`, and `ORION_JWT_SECRET`.
- [x] Confirm the SQLite Docker volume persists after container restart.
- [x] Confirm generated OpenAPI and SDK are current after route/contract changes.

## Priority 2 - Agent Install Smoke

- [x] Make duplicate monitor registration idempotent so Agent can recover from local state loss.
- [x] Run the Agent install script against a local Core URL.
- [x] Confirm Agent creates and reuses local SQLite state.
- [x] Confirm Agent registration creates one stable Core agent.
- [x] Confirm monitor registration creates expected Core monitors.
- [x] Confirm Agent sends system reports on its configured interval.
- [x] Confirm Agent sends monitor reports on each monitor interval.
- [x] Confirm Agent restart does not duplicate registration.
- [x] Confirm uninstall removes service files cleanly and handles state intentionally.

## Priority 3 - Auth And Sensitive Data

- [x] Wire Login page to Core auth.
- [x] Add username input to Login page.
- [x] Add sign-out action in the app header.
- [x] Confirm frontend auth is either fully enabled or clearly disabled.
- [x] Confirm partial frontend auth configuration fails loudly.
- [x] Confirm expired or invalid frontend tokens show a usable login/session state.
- [x] Confirm sign out clears local auth state in a running browser.
- [x] Confirm frontend-facing Agent responses never include Agent tokens.

## Priority 3.5 - Agent Runtime Risks

- [x] Make maintenance mode fail clearly or retry when Core cannot be updated.
- [x] Add timeout to external geolocation lookup.
- [x] Decide whether retry queue persistence is needed for first deploy.
- [x] Document Docker monitor permissions for the systemd `orion` user.
- [x] Fix macOS uninstall paths for config/state cleanup.

## Priority 4 - UI Release Pass

- [x] Verify incident list pagination, filters, summary cards, and detail tabs.
- [x] Verify agent list pagination, filters, summary cards, detail tabs, and monitor expansion.
- [x] Verify monitor list pagination, filters, summary cards, and detail tabs.
- [x] Verify alerts page channel, rule, and delivery filters.
- [ ] Verify event log pagination and filters.
- [ ] Verify settings data lifecycle read/update/manual actions.
- [ ] Verify empty, loading, and error states on every main page.
- [ ] Verify URL query state for tabs, filters, and pagination.

## Priority 5 - Documentation Cleanup

- [x] Remove stale docs that claim implemented routes are missing.
- [x] Update first deploy instructions with exact environment variables.
- [ ] Update Agent install/upgrade instructions after smoke testing.
- [x] Add a short first-run checklist for Core plus one Agent.
- [x] Document backup/restore expectations for the Docker volume.
- [x] Update `docs/plans/README.md` to include this first deploy checklist.

## Priority 6 - Release Packaging

- [ ] Decide first deploy image tag format.
- [ ] Add a reproducible Core Docker build command for tagged images.
- [ ] Add a reproducible Agent binary build command.
- [ ] Document version compatibility between Core and Agent.

## Nice To Have After First Deploy

- [ ] Add manual incident acknowledge/resolve actions.
- [ ] Add alert channel test action.
- [ ] Add Core runtime metrics for report write rate, ingestion latency, DB size, and slow requests.
- [ ] Add automated archive/rollup scheduling.
- [ ] Add Agent update bookkeeping in local state.
- [ ] Add durable Agent report spool if first deploy usage shows report gaps during restarts or long Core outages.
