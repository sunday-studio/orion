# CI Baseline

Orion uses `.github/workflows/ci.yml` for pull request and `main` branch validation.

The workflow is path-aware:

- Server changes run Go formatting checks and `go test ./...` in `apps/agent`.
- Core changes run `make core-coverage`, upload package/function coverage artifacts, and build the
  Core API and worker binaries after Go formatting checks pass.
- Console changes install dependencies with pnpm, generate the local SDK from committed OpenAPI,
  run format and lint checks, and run the Console build.
- API or generated-contract changes regenerate OpenAPI before generating the local Console SDK.
  CI then fails if regenerated Core OpenAPI and Swagger files are not committed.
- Deploy and documentation changes run repository smoke checks, including shell syntax and Docker
  Compose config validation.

Release-only jobs stay separate:

- `.github/workflows/core-image.yml` publishes multi-architecture Core images to GHCR.
- `.github/workflows/agent-binaries.yml` builds and publishes multi-platform Server release assets.

Those release jobs are intentionally manual because they publish external artifacts and require
explicit version inputs.

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

Coverage thresholds are still deferred. A raw percentage would be misleading until generated docs
and integration-heavy packages have agreed package-level interpretation.
