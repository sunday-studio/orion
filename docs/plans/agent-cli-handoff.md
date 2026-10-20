# Agent CLI and terminal UI: completion handoff

> Superseded for status by [agent-cli-remaining.md](agent-cli-remaining.md), which lists each board, the test that
> proves it, and what is left. This file keeps the original table and validation commands.

Updated 2026-10-19 from the public **CLI and agent** design board. The board is the source of truth;
the six checked-in HTML artboards are an older subset.

## Current state

All 16 artboards on the board have an implemented command or state:

| Board | Implementation |
|---|---|
| Install script | Plan-only dry run, verified download, concise completed steps, enrollment, Doctor box, failure exit, and same-version no-op in `deploy/scripts/agent-bootstrap.sh` |
| Setup wizard | Three-step connect, discover/select, and review flow; Add server token preflight; unified config diff; non-interactive flags |
| Doctor | Streaming checks, exact fixes, summary and non-zero failure exit; stable per-check JSON |
| Status | Six ordered connection states, maintenance reason/end, actionable unhealthy states, normal-user control socket, flat JSON schema 2 |
| Log viewer | Follow, pause, snapshot, search/filter, empty/error states, live daemon-state chip, and offline spool fallback |
| Help and errors | Intent-grouped 80-column help, full uncommon-command discovery, styled diagnostics, exit codes 1/2/3 |
| Command map | Every documented command is reachable; command groups reject unknown subcommands with exit 2 |
| Start/stop/restart/run | Preflight and recovery output; clean `run --once` output without internal runtime logs |
| Maintenance | Timed down/up, 720-hour bound, local-only unenrolled state, Core update, status detail |
| Config | Effective sources, compact unified diff with context, strict validation, edit-before-save and daemon reload |
| Discover and usage | Read-only discovery ids/confidence/notes and bounded-width external resource usage; JSON output |
| Update and rollback | Checksum verification, atomic swap, health check, automatic rollback, same-version and rollback states |
| Enroll and credentials | Add server vs replacement-token distinction, safe token apply, authorized reconnect, compact state init |
| Emit and run-job | Validate before network access, unchanged child output, shipping warnings on stderr, exact exit-code pass-through |
| Status states | Not enrolled, maintenance, waiting, stale, connected, and not running with a recovery command |
| Log states | Following, paused, snapshot, no matches, spool errors, daemon connection state, faint trace/debug levels |

The setup and credential paths distinguish the two credentials deliberately:

- `orn_enr_...` is a one-time **Add server** authorization for `setup`, `enroll`, or `reconfigure`.
- A 64-character Server replacement token is applied with `token apply`; it is never sent to
  registration.
- `reconfigure` validates the Add server token before stopping the service, reconnects the known
  machine without clearing working local state first, and refuses to move enrolled state silently
  between Core instances.

## Validation

Run before release or handoff:

```sh
cd apps/agent
GOCACHE=/private/tmp/orion-go-cache go test ./...
GOCACHE=/private/tmp/orion-go-cache go build -o /tmp/orion-tui-work/orion-agent .
cd ../..
GOCACHE=/private/tmp/orion-go-cache make agent-installer-test
make code-line-limit
```

Screen snapshots are in `apps/agent/internal/cli/testdata/screens`; JSON shape snapshots are in
`apps/agent/internal/cli/testdata/golden`. The user owns live visual acceptance and additional test
authoring.

## Intentional limits

- Status does not claim a release is `(latest)` because the local Agent has no release-index lookup.
- Disk monitor rows show the health word until the control protocol carries the exact measured value.
- The in-place `update` command verifies SHA256 but not cosign; the front-door installer verifies the
  release signature when cosign is available and can require it.
- The optional OTLP relay is not part of the 16-board terminal UI and is not required when apps send
  their own telemetry directly.
- Running the Agent as a container image or DaemonSet remains outside this terminal-UI scope.

## Completed local commits

- `6fb6687` — setup rebuilt as a verified three-step flow.
- `51b45a0` — runtime status, logs, maintenance, update, command-group and emit alignment.
- `65cdb85` — safe credential recovery and script command behavior.
- `1ec71be` — concise, idempotent installer flow.

The final board-audit changes after those commits must also be committed before handoff.
