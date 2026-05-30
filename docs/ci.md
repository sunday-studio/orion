# CI Baseline

Orion uses `.github/workflows/ci.yml` for pull request and `main` branch validation.

The workflow is path-aware:

- Server changes run `go test ./...` in `apps/agent`.
- Core changes run `go test ./...` in `apps/core` and build the Core API and worker binaries.
- Console changes install dependencies with pnpm and run the Console build.
- API or generated-contract changes regenerate OpenAPI and the Console SDK, then fail if generated
  files are not committed.
- Deploy and documentation changes run repository smoke checks, including shell syntax and Docker
  Compose config validation.
- The release readiness job aggregates path-aware job results and fails when any required gate fails
  or is cancelled.

Release-only jobs stay separate:

- `.github/workflows/core-image.yml` publishes multi-architecture Core images to GHCR.
- `.github/workflows/agent-binaries.yml` builds and publishes multi-platform Server release assets.

Those release jobs are intentionally manual because they publish external artifacts and require
explicit version inputs.

## Release Readiness

`make release-readiness` runs the local blocking gate for Server tests, Core tests, Console build,
and repository smoke checks. Contract-changing PRs must also run `make generated-contracts-check`.

The full matrix and warning classification rules live in
[Release readiness gate](deployment/release-readiness.md).

## Coverage

The README shows the live CI workflow badge. A coverage badge is deferred until Orion publishes
coverage reports from CI to a durable provider or GitHub Pages artifact. Until then, adding a static
coverage badge would be misleading.
