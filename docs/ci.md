# CI Baseline

Orion uses `.github/workflows/ci.yml` for pull request and `main` branch validation.

The workflow is path-aware:

- Server changes run `go test ./...` in `apps/agent`.
- Core changes run `make core-coverage`, upload package/function coverage artifacts, run race
  detection on the Core service and worker packages, run the Core modernization lint gate, run
  `govulncheck`, and build the Core API and worker binaries.
- Console changes install dependencies with pnpm and run the Console build.
- API or generated-contract changes regenerate OpenAPI and the Console SDK, then fail if committed
  Core generated files drift or SDK generation stops producing `apps/console/src/orion-sdk/index.ts`.
- Deploy and documentation changes run repository smoke checks, including shell syntax and Docker
  Compose config validation.
- The release readiness job aggregates path-aware job results and fails when any required gate fails
  or is cancelled.

Release-only jobs stay separate:

- `.github/workflows/core-image.yml` publishes multi-architecture Core images to GHCR.
- `.github/workflows/agent-binaries.yml` builds and publishes multi-platform Server release assets.

Those release jobs are intentionally manual because they publish external artifacts and require
explicit version inputs.

## Core backend verification

Run these commands before opening a Core backend PR:

```sh
make core-test
make core-coverage
make core-race
make core-modernize-check
make core-vulncheck
make core-contract-check
make core-build CORE_OUTPUT=/tmp/orion-core
make core-worker-build CORE_WORKER_OUTPUT=/tmp/orion-core-worker
```

`make core-backend-verify` runs the same local bundle.

The modernization lint gate uses `golangci-lint` with only the `modernize` linter enabled. CI
reports only new pull request issues, and the local Makefile target reports issues introduced after
the merge base with `main`. Existing modernization findings stay with the dedicated Core
modernization cleanup goal instead of blocking unrelated CI changes.

The generated-contract job remains the OpenAPI drift and Console SDK generation check. `make
core-contract-check` is the narrower backend-only drift check for generated Core Swagger docs and
`apps/core/openapi.yaml`.

## Release Readiness

`make release-readiness` runs the local blocking gate for Server tests, Core tests, Console build,
and repository smoke checks. Contract-changing PRs must also run `make generated-contracts-check`.

The full matrix and warning classification rules live in
[Release readiness gate](deployment/release-readiness.md).

## Coverage

Core pull requests publish `apps/core/coverage.out` and `apps/core/coverage-summary.txt` as the
`core-coverage` artifact. The summary comes from `go tool cover -func`, so reviewers can see package
and function-level movement without adding a durable badge provider.

Core backend source changes are also checked for test intent. If a pull request changes Core
backend Go code, module files, or SQL migrations without changing any `apps/core/**/*_test.go` file,
CI fails unless the PR body fills in `No Core backend test changes because:` with a concrete
rationale. This gate is intentionally scoped to pull requests and Core backend files so
documentation, deploy, Console-only, and generated-contract-only changes do not inherit backend
test policy.

The Console SDK stays ignored locally, so clean CI checkouts generate it before TypeScript resolves
`@/orion-sdk`. Backend contract changes regenerate OpenAPI first; frontend-only changes generate the
SDK from the committed OpenAPI file.

Coverage thresholds are still deferred. The README shows the live CI workflow badge, but a coverage
badge is deferred until Orion publishes coverage reports from CI to a durable provider or GitHub
Pages artifact. Until then, adding a static coverage badge would be misleading.
