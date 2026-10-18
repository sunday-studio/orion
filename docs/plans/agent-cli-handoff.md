# Agent and CLI: handoff

Written 2026-10-18 for the next agent. Plan: [agent-cli-plan.md](agent-cli-plan.md). Work in the main checkout (`/Users/casprine/Desktop/vendor/sunday-studio/orion`, branch `main`), never a worktree. `git pull --rebase origin main`, commit small, `git push origin HEAD:main`, never force. The user approved pushing **reviewed** batches: get an independent review (a separate subagent using `.agents/personas/staff-engineer-review.md`) before each push. Code-level checks only (build, vet, `-race` tests, goldens); the user does all screenshots and live checks. Do not message other threads except one "F9 ready: <commit>" to the coordinator at the end (and "agent monitors done: <commit>" if it was not sent: it was not).

## What is on `origin/main` (reviewed)

- Static CGO-free builds, pure-Go state store, control socket, zero-config and `config.d`, `enroll`, installer v2 (cosign and checksum verification, `--user`, uninstall), safe `update` with rollback, audit fixes, performance budgets, design-token theme (`docs/design/tokens.json` to `internal/ui/theme_gen.go`), discovery and `setup`, passive per-app metrics (`appres`, `usage`), `doctor`, `status`, help and error screens.
- Agent monitors (F4): the daemon pulls `POST /v1/agents/{id}/monitor-definitions/sync` after registering and every minute, maps disk/memory/process/port checks to monitors (`internal/remotemon`), persists the last good set in `remote-monitors.json`.
- Screens rebuilt to the artboards with literal-match tests: `doctor`, `status`, root help, config error. Goldens live in `apps/agent/internal/cli/testdata/screens` (`go test ./internal/cli -run Screen -update-screens` rewrites them; the artboard-literal tests must keep passing without update).

## Committed locally, NOT pushed, NOT reviewed (batch B)

Run the full suite, get a review, then push.

- Log shipper (Phase E): `internal/logship` (rotation-aware tailer, multiline, edge redaction, bounded spool with cursor, NDJSON sender with Retry-After, journald and docker stream sources, offsets in the state DB) wired into the daemon (`internal/cli/logs-wire.go`, config `logs:` with `sources[]`, per-source `ingest_key_file` or `ingest_key_env`). Sends to `{core}/v1/ingest/logs` per `docs/product-logs.md`.
- `logs` command: shipped logs from the spool through the control socket (`GET /v1/spool`), works offline; `--follow` is a bubbletea viewer (`logs-tui.go`) on a TTY and a plain tail otherwise; `--source file|system` keeps the old agent-log behavior; `--name`, `--search`, `--level`. Screen rendering is `logs-view.go`, matched to `cli-logs.html` in `logs-view_test.go`.
- `emit` and `run-job` (`emit.go`): one-shot entry; run a command, ship its output and a `job.finished` or `job.failed` entry, ping `/v1/heartbeats/{token}/success|failure`, pass the exit code through.
- Re-run `cd apps/agent && CGO_ENABLED=0 go test -race -timeout 900s ./...` (about 3 minutes; `internal/logship` and `internal/state` are slow because every spool append fsyncs). A run was started in the background and its result was not read: do not assume it passed.

## Remaining, in order

1. **Review and push batch B.** Review-bound items: the doctor has no check for log sources and keys yet (add one: key file readable, endpoint reachable); document the `logs:` section and `emit`/`run-job` in `docs/agent-core-contract.md` or a config reference.
2. **`cli-setup` artboard** (`docs/design/artboards/cli-setup.html`): three steps (connect, choose, review), step counter in the title, focused field with a green `┃` bar, confidence labels, unified diff in the review step, "Applies without restarting the agent." Today `internal/cli/setup-wizard.go` is a huh form with a different layout. Non-interactive flags named in the artboard: `--core-url`, `--token`, `--accept-discovered`, `--yes`. Needs literal-match tests like the other screens.
3. **`cli-install` artboard** (`cli-install.html`): the installer's output (`deploy/scripts/agent-bootstrap.sh`): Plan block with `●` rows, one `✓` line per step, spinner for "Waiting for the first report...", a Doctor box, "Done in N s. Open <core>/servers/<name>", `--dry-run` prints only the plan, a tampered download stops at Verify with a red `✗` and expected and actual hash, re-running says it is a no-op. It is a bash script, so test with shell tests in `deploy/scripts/` (existing: `agent-bootstrap-test.sh`, `test-agent-bootstrap-verify.sh`).
4. **Remaining plan items**: release pipeline and signing (F1: `.github/workflows/agent-binaries.yml`, SBOM, cosign), docs and generated config reference (F3), OTLP relay (E5, optional, last). **F2, running the agent as a container image or DaemonSet, is deferred to the very end (D-031).**
5. Send the coordinator exactly one message: `F9 ready: <commit>` with a table of the six CLI screens and what is done or deferred.

## Deliberate deviations from the artboards (tell the user)

- Status prints no `(latest)` after the version (the agent cannot know the latest release) and shows up/slow/down words instead of "91%" for the disk row.
- Root help adds a trailing `More:` line so `enroll`, `discover`, `usage`, `token`, `maintenance`, `version`, `run`, `emit`, `run-job` stay discoverable.
- Config errors have no `Docs:` line (no config reference page exists yet to link).
- The artboard's "8 passed" implies a Spool check, which was added.

## Rules and gotchas

- Files under 500 lines (`make code-line-limit`); new files are lowercase kebab-case.
- Exit codes: 0 ok, 1 failed, 2 usage, 3 needs elevation. `--json` documents carry `schema_version`; goldens in `internal/cli/testdata/golden` (`go test ./internal/cli -run GoldenJSON -update`). Doctor JSON is schema 3.
- zsh does not split `$A`-style variables; use shell functions in instructions to the user.
- Platforms are Linux and macOS only (D-014). Maat is removed from the repo.
- Known accepted gaps from reviews: `update` checks the checksum but not the cosign signature; the installer exits 0 when `doctor` fails; group commands with an unknown subcommand exit 0; cobra's "Did you mean" text is embedded in unknown-command errors; a tailer misses a file that rotates twice within one 250 ms poll.
- Core gaps to ask backend about directly, not via the coordinator: `fail_after` is passed through as monitor meta but Core does not enforce it; gzip request bodies are decoded only on the log-ingest endpoint (so `transport.compression: gzip` stays opt-in).
