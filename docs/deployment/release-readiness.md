# Release Readiness Gate

This gate decides whether a change can be considered ready for a release candidate. It is stricter
than "the branch builds" and narrower than a full production rollout.

## Decision Rule

A release candidate is ready when every blocking row in the matrix is green, every warning is
classified, and the pull request records the evidence. Advisory warnings can ship only when the PR
names the owner, risk, and follow-up ticket.

## Verification Matrix

| Area | Blocking gate | Evidence | Local command | CI coverage |
| --- | --- | --- | --- | --- |
| Server daemon | Server tests pass for the changed branch. | Test output or CI job URL. | `make agent-test` | `Server tests` |
| Core API and worker | Core tests pass and Core binaries build. | Test/build output or CI job URL. | `make core-test && make core-build && make core-worker-build` | `Core tests` |
| Console | Generated SDK, TypeScript, and Vite build pass. | Build output or CI job URL. | `make console-build` | `Console build` |
| Browser smoke | Console browser smoke passes when Console, Core API, or generated API client changes. | Playwright output or CI job URL. | `cd apps/console && pnpm run test:e2e` | `Console E2E` |
| Generated contracts | OpenAPI and generated Console SDK have no drift after regeneration. | Clean `git diff` output. | `make generated-contracts-check` | `Generated contracts` |
| Deployment assets | Shell scripts parse and Docker Compose renders with required secrets. | Command output or CI job URL. | `make repository-smoke` | `Repository smoke` |
| Release packaging | Core image and Server binary workflows remain manual, versioned, and same-tag compatible. | Reviewer checklist and workflow diff. | Inspect `.github/workflows/core-image.yml` and `.github/workflows/agent-binaries.yml` | Manual PR review |
| Documentation | Deployment, release, and PR checklist changes stay consistent with supported self-hosted flow. | PR checklist and changed docs. | Inspect `docs/deployment/` and `README.md` links | Manual PR review |

Run the local release gate before opening a release-candidate PR. This includes Server tests, Core
tests, Core API and worker builds to `/tmp`, Console build, and repository smoke checks:

Install Console dependencies first in a fresh checkout because `make console-build`,
`make build-static`, `make generated-contracts-check`, and `make release-readiness` generate or
build the Console SDK:

```sh
cd apps/console && pnpm install --frozen-lockfile
```

```sh
make release-readiness
```

Run `make generated-contracts-check` as an additional blocking gate when Core route annotations,
Core API models, OpenAPI output, the generated SDK, or Console API usage changes.

## Warning Classification

| Classification | Release effect | Examples | Required PR note |
| --- | --- | --- | --- |
| Blocking | Do not merge or tag the release candidate. | Failing tests, generated contract drift, broken Docker Compose config, missing required deployment secret docs, incompatible Core/Server version guidance. | Mark the checklist item failed and link the fix commit or follow-up PR. |
| Conditional | May merge only with an explicit owner and mitigation. | Browser smoke flakes with a rerun pass, known unsupported monitor edge case, release workflow warning that does not affect produced artifacts. | Name the owner, impact, expiry date, and Maat ticket. |
| Advisory | May merge when documented. | Cosmetic documentation gap, non-release TODO, planned future hardening, optional local tooling warning. | Explain why it is not release-blocking and link any follow-up ticket. |

Unclassified warnings are blocking. Treat a warning as unclassified when the PR does not say whether
it is blocking, conditional, or advisory.

## Known Release Warnings

These warnings are classified for the current release gate:

| Warning | Classification | Why it is not blocking | Escalates to blocking when |
| --- | --- | --- | --- |
| `github.com/shoenig/go-m1cpu` emits Darwin CGO variable-length-array warnings during Server tests. | Advisory | The Server tests pass and the warning comes from a third-party dependency compile step. | It becomes a compile failure, appears on Linux release builds, or masks a failing Server test. |
| `swag` warns that it failed to evaluate Go runtime constant `mProfCycleWrap` while parsing dependencies. | Advisory | OpenAPI generation completes and the generated contract drift check passes. | OpenAPI generation fails, route documentation is missing, or generated contract files drift unexpectedly. |
| Orval prints `import.meta` target warnings while generating the Console SDK. | Advisory | SDK generation and the Vite production build pass after generation. | Generated SDK output changes in a way that breaks TypeScript, runtime API base URL handling, or browser smoke. |
| Vite warns that the main Console chunk is larger than 500 kB. | Conditional | The production build succeeds and the first release has no documented bundle-size budget. | Console load time becomes a release criterion, browser smoke exposes load failures, or a reviewer sets a bundle budget for the release. |
| `pnpm install` warns that dependency build scripts were ignored. | Advisory | The frozen install, SDK generation, and production build pass in the isolated worktree. | A package requires an ignored build script for the build, tests, or browser smoke to pass. |

## Manual Release Checks

Before tagging a release, confirm:

- The Core image workflow and Server binary workflow are both run with the same semantic version.
- The first self-hosted deploy path in `docs/deployment/first-run-checklist.md` still matches the
  published artifacts.
- The Core and Server compatibility note in `docs/deployment/release-packaging.md` still matches the
  release notes.
- Any conditional warning has a named owner and follow-up ticket.

## Release Candidate Notes

Each release-candidate PR should include:

- the `make release-readiness` result;
- any additional contract, browser, Docker, or packaging evidence needed by the matrix;
- warning classifications;
- the intended release version or a statement that no tag will be cut from the PR.
