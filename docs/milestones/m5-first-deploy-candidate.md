# M5: First Deploy Candidate

## Goal

Finish the checklist needed to test Orion as a first self-hosted deploy candidate.

## Completed

- Interval-based stale detection for servers and monitors.
- Server registration sends the Server reporting interval.
- Monitor registration sends each monitor reporting interval.
- Core Docker build and Docker Compose smoke path.
- Core serves Console static assets from the runtime image.
- Docker Compose requires frontend auth environment variables.
- Core health endpoint and Console login smoke checks.
- SQLite Docker volume persistence check.
- Server install smoke checks for local state, stable Server registration, monitor registration, reporting intervals, restart behavior, and uninstall behavior.
- Frontend auth sign-in/sign-out and expired session behavior.
- Frontend-facing Server responses avoid sensitive Server tokens.
- Server runtime risk cleanup for maintenance updates, geolocation timeout, Docker monitor permissions, macOS uninstall paths, and retry queue decision.
- UI release pass for incidents, servers, monitors, alerts, event logs, settings, page states, and URL state.
- Server install/upgrade docs, first-run checklist, backup/restore docs, and deployment docs.
- Release packaging commands for tagged Core images and Server binaries.
- Automated data lifecycle archive and rollup scheduling in Core.

## Release Packaging

- First deploy tags use semantic versions such as `v0.1.0`.
- Core image build:

```sh
VERSION=v0.1.0 make docker-build
```

- Server binary build:

```sh
VERSION=v0.1.0 make agent-build
```

- Core and Server should run the same release tag for the first deploy.

## Verification

- `npm run lint`.
- `npm run build`.
- `go test ./internal/api -run TestListOrionEvents`.
- `go test ./internal/api -run TestDataLifecycle`.
- `go test ./...` in `apps/agent`.
- `GOCACHE=/private/tmp/orion-go-cache make agent-build VERSION=v0.1.0-smoke`.
- `make docker-build VERSION=v0.1.0-smoke`.

## Deferred Until After First Deploy

- Manual incident acknowledge/resolve actions.
- Alert channel test action.
- Core runtime metrics for report write rate, ingestion latency, DB size, and slow requests.
- Server update bookkeeping in local state.
- Durable Server report spool if real usage shows unacceptable report gaps.

## Next

- Run the first real deployment test.
- Keep new issues in a short testing note or issue list instead of reviving completed plans.
