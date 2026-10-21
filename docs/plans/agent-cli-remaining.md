# Agent CLI: functionality and designs left

Updated 2026-10-21. Method: one functionality at a time, each built to its design board text,
checked by literal-match tests, reviewed by an independent agent, then pushed to `main`.

Note: the public design canvas needs a signed-in session; agents cannot read it. Boards are
taken from `apps/agent/internal/cli/testdata/design/*.txt` (extracted earlier) and the checked-in
`docs/design/artboards/cli-*.html`. Visual acceptance stays with the user.

## Done to the board

Most boards have a literal-match test against their text in `apps/agent/internal/cli/testdata/boards/`
(extracted from the design canvas). Discover and Status match the earlier extracts in
`testdata/design/`. The installer is covered by its shell tests (`deploy/scripts/agent-bootstrap-test.sh`,
`test-agent-bootstrap-verify.sh`), which assert output fragments rather than the whole board. Output modes and
exit codes are documentation only. Each screen was reviewed by an independent agent before it was pushed.
Where the code deliberately differs from a board, the test or this table says why.

| Board | Where | Deviations noted in tests |
|---|---|---|
| Auto-detect, Discover | `discover-render.go`, `discover-pick.go` | none |
| Status, `--watch`, plain, JSON | `status-render.go`, `status-watch.go` | none |
| Setup (connect, choose, scan settings, review, applied, non-interactive, accessible) | `setup-flow*.go`, `setup-accessible.go` | none |
| Enroll and credentials | `credentials-render.go` | none |
| Install script | `deploy/scripts/agent-bootstrap.sh` (shell tests) | exit code 2 for a failed verification (documented contract) instead of the board's 1 |
| Doctor | `doctor-render.go`, `internal/doctor` | failing view lists every warning and no passes |
| Log viewer | `logs-render.go`, `logs-tui.go` | the spool does not record a send time, so the drawer says "sent to Core" without one |
| Help and errors | `help-render.go`, `errors-render.go` | the "More" line also lists the commands the board omits |
| Start, stop, restart, run, service install | `service-flow.go`, `operate-render.go` | none |
| Maintenance | `maintenance-*.go` | none |
| Update and rollback | `update-render.go`, `update.go` | "the previous version is running again" instead of "kept reporting the whole time" |
| Config show, diff, validate, edit | `config-*.go` | monitor count is the number listed |
| Emit and run-job | `emit-render.go` | none |
| Usage, `--watch` | `usage-render.go`, `usage-watch.go` | none |
| Output modes, exit codes | `docs/deployment/server-cli-cheatsheet.md` | documentation only, no test |

## Left

| # | What | Why it is left |
|---|---|---|
| 1 | Cutting a release | the workflow (`.github/workflows/agent-binaries.yml`) already builds, signs with keyless cosign, writes an SBOM and publishes the installer; running it is your decision, after everything is verified locally |
| 2 | Console views for app resources | Core stores and serves the latest measurements (`GET /v1/agents/{id}/app-resources`); orion:design places Docker CPU, memory and storage in the server page and orion:web builds it |
| 3 | Creating a log source from the Console | recommended to skip: it needs Core to hand a secret log key to the agent; today the person pastes it on the server |
| 4 | Publishing the agent image | the image builds and is tested (`make agent-image`, `docs/deployment/agent-container.md`); the release workflow does not push it yet. A Kubernetes DaemonSet and the Docker socket path on real Linux are untested |
| 5 | Visual acceptance in real terminals (TTY redraws, raw-mode prompts, Linux) | the user owns it; agents cannot drive a real terminal |

Done since the last update: Console actions for the Detected panel (Core request queue, agent poller and applier, service, container and pm2 checks), markdown for release notes and `help <topic>` with a small built-in renderer (Glamour added about 10 MB and was dropped), Core ingestion of app resources, and the agent container image.

Decided not to do: VHS tapes for README and website visuals (2026-10-21). Plain-text output in the docs is enough and cannot go stale.
