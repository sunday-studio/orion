# CI Baseline

Orion uses `.github/workflows/ci.yml` for pull request and `main` branch validation.

The workflow is path-aware:

- Server changes run `go test ./...` in `apps/agent`.
- Core changes run `go test ./...` in `apps/core` and build the Core API and worker binaries.
- Console changes install dependencies with pnpm and run the Console build.
- API or generated-contract changes regenerate OpenAPI and the Console SDK, then fail if generated files are not committed.
- Deploy and documentation changes run repository smoke checks, including shell syntax and Docker Compose config validation.

Release-only jobs stay separate:

- `.github/workflows/core-image.yml` publishes multi-architecture Core images to GHCR.
- `.github/workflows/agent-binaries.yml` builds and publishes multi-platform Server release assets.

Those release jobs are intentionally manual because they publish external artifacts and require explicit version inputs.
