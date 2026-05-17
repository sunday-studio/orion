# First Deploy Todo

This checklist tracks the work needed before Orion is ready for a first self-hosted deploy candidate.

## Priority 0 - Commit Current Work

- [x] Implement interval-based stale detection for agents and monitors.
- [x] Send agent reporting interval during Agent registration.
- [x] Send monitor reporting interval during monitor registration.
- [x] Regenerate OpenAPI and Console SDK from Core annotations.
- [x] Update Agent/Core contract and architecture docs for interval-aware stale rules.
- [ ] Commit interval-based stale detection changes.

## Priority 1 - Core Deploy Smoke

- [ ] Build the Core Docker image with `make docker-build`.
- [ ] Start Core with Docker Compose using `make docker-up`.
- [ ] Confirm `/health` returns healthy.
- [ ] Confirm the Console loads from the Core container.
- [ ] Confirm login works with `ORION_ADMIN_USERNAME`, `ORION_ADMIN_PASSWORD`, and `ORION_JWT_SECRET`.
- [ ] Confirm the SQLite Docker volume persists after container restart.
- [ ] Confirm generated OpenAPI and SDK are current after route/contract changes.

## Priority 2 - Agent Install Smoke

- [ ] Run the Agent install script against a local Core URL.
- [ ] Confirm Agent creates and reuses local SQLite state.
- [ ] Confirm Agent registration creates one stable Core agent.
- [ ] Confirm monitor registration creates expected Core monitors.
- [ ] Confirm Agent sends system reports on its configured interval.
- [ ] Confirm Agent sends monitor reports on each monitor interval.
- [ ] Confirm Agent restart does not duplicate registration.
- [ ] Confirm uninstall removes service files cleanly and handles state intentionally.

## Priority 3 - Auth And Sensitive Data

- [ ] Confirm frontend auth is either fully enabled or clearly disabled.
- [ ] Confirm partial frontend auth configuration fails loudly.
- [ ] Confirm expired or invalid frontend tokens show a usable login/session state.
- [ ] Confirm sign out clears local auth state.
- [ ] Confirm frontend-facing Agent responses never include Agent tokens.

## Priority 4 - UI Release Pass

- [ ] Verify incident list pagination, filters, summary cards, and detail tabs.
- [ ] Verify agent list pagination, filters, summary cards, detail tabs, and monitor expansion.
- [ ] Verify monitor list pagination, filters, summary cards, and detail tabs.
- [ ] Verify alerts page channel, rule, and delivery filters.
- [ ] Verify event log pagination and filters.
- [ ] Verify settings data lifecycle read/update/manual actions.
- [ ] Verify empty, loading, and error states on every main page.
- [ ] Verify URL query state for tabs, filters, and pagination.

## Priority 5 - Documentation Cleanup

- [ ] Remove stale docs that claim implemented routes are missing.
- [ ] Update first deploy instructions with exact environment variables.
- [ ] Update Agent install/upgrade instructions after smoke testing.
- [ ] Add a short first-run checklist for Core plus one Agent.
- [ ] Document backup/restore expectations for the Docker volume.

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
