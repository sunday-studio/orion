# CI Baseline

Orion uses `.github/workflows/ci.yml` for pull request and `main` branch validation.

The workflow is path-aware:

- Server changes run `go test ./...` in `apps/agent`.
- Core changes run `go test ./...` in `apps/core`, run race detection on the Core service and
  worker packages, run the Core modernization lint gate, run `govulncheck`, and build the Core API
  and worker binaries.
- Console changes install dependencies with pnpm and run the Console build.
- API or generated-contract changes regenerate OpenAPI and the Console SDK, then fail if committed
  Core generated files drift or SDK generation stops producing `apps/console/src/orion-sdk/index.ts`.
- Deploy and documentation changes run repository smoke checks, including shell syntax and Docker
  Compose config validation.

Release-only jobs stay separate:

- `.github/workflows/core-image.yml` publishes multi-architecture Core images to GHCR.
- `.github/workflows/agent-binaries.yml` builds and publishes multi-platform Server release assets.

Those release jobs are intentionally manual because they publish external artifacts and require
explicit version inputs.

## Core backend verification

Run these commands before opening a Core backend PR:

```sh
make core-test
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

## Coverage

The README shows the live CI workflow badge. A coverage badge is deferred until Orion publishes
coverage reports from CI to a durable provider or GitHub Pages artifact. Until then, adding a static
coverage badge would be misleading.
