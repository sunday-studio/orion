# Agent CLI walkthrough

Walk every flow on the design canvas (CLI and agent page) against the real binary. Each step says
what to run, what the board shows, and what to compare. Tick as you go; write down anything that
looks different from the board and the exact command.

Legend: **sandbox** works with the throwaway setup below, no root. **host** needs a real server or
root (a service manager); do it on a spare Linux box or VM, or skip.

## Sandbox

Three terminals. Everything lives in `/tmp/orion-play`, nothing is installed.

```sh
# terminal 1: build, then run Core
mkdir -p /tmp/orion-play && cd "$(git rev-parse --show-toplevel)"
(cd apps/core && CGO_ENABLED=1 go build -o /tmp/orion-play/core ./cmd/api)
(cd apps/agent && go build -o /tmp/orion-play/oa .)
ORION_ALLOW_INSECURE_NO_AUTH=true ORION_DATA_DIR=/tmp/orion-play/data ORION_PORT=18990 /tmp/orion-play/core
```

```sh
# terminals 2 and 3: the agent command, and the Core address
oa() { ORION_LOG_PATH=/tmp/orion-play/agent.log /tmp/orion-play/oa --config /tmp/orion-play/config.yaml --state /tmp/orion-play/state.db --socket /tmp/orion-play/a.sock "$@"; }
CORE=http://127.0.0.1:18990
```

**One host per Core.** Core identifies a host by its machine id, so this machine can enrol only once
per Core database; a second enrolment fails with a 409. That is why the order below matters: do the
fresh-host flows (parts 1 and 2) first, then enrol and run the agent for everything after.

**To start over:** stop the agent (Ctrl+C in terminal 2) and Core (terminal 1), then
`rm -rf /tmp/orion-play/data /tmp/orion-play/config.yaml /tmp/orion-play/state.db /tmp/orion-play/a.sock`
and start Core again.

In this sandbox the Core runs with authentication off and accepts any token, `oa doctor` reports
"Service is not installed" (no service is installed, so it exits 1), and `emit` / `run-job` have no
ingest key. These are expected here; each affected row says so.

## 1. Setup, before the host is enrolled (board: CLI, setup)

| Screen | Run | Compare |
|---|---|---|
| Not running | `oa status` first, before anything else | Chip "Not running", "the service is not installed", next `orion-agent service install`. The "Not enrolled" chip needs a running daemon with no registration, which this sandbox cannot produce (with authentication off the daemon registers itself); host only |
| Connect | `oa setup` (token: any text, for example `play`) | Step 1 of 3, Core address with reachable check, masked one-time token, key hints |
| Choose | continue the same flow | Grouped panels (Services, Containers, Ports, Disks), `already monitored` rows, confidence column |
| Scan settings | open it from Choose | Kinds on/off, report toggle |
| Review | continue | Config diff with tinted lines, one confirm |
| Applied | confirm | Result card, next step |
| Accessible | `ACCESSIBLE=1 oa setup` on a terminal | Plain line prompts, no animation (a flag-driven run such as `--dry-run` never prompts) |
| Non-interactive | `oa setup --core-url $CORE --accept-discovered --yes --init-state` | Checklist: Scanned, Accepted, Skipped, Config written, State |

The interactive flow enrols the host when you confirm Apply (the Connect step only checks Core and the token), so after it part 2's enrol rows show "Already enrolled". To see the enrol screens fresh, start over (see above) and use the
non-interactive setup, which only writes the config.

## 2. Enrol, then start the agent

| Screen | Run | Compare |
|---|---|---|
| Enroll | `oa enroll --core-url $CORE --token play` | Reachable, Token accepted, Enrolled as … |
| Enroll again | run it again | "Already enrolled", nothing to do |
| Waiting | run `oa status` immediately after `oa run` starts | "Waiting, first report is due within 30 s"; the window is only a few seconds, so it is easy to miss. Before the daemon runs, `oa status` says "not running" |
| Start the agent | terminal 2: `oa run` (enrol first, as above: with authentication off `oa run` also registers on its own, and then `oa enroll` only says "Already enrolled") | Checklist, then one dim line "Running and reporting" (not on the board) |
| Run once | stop it first, then `oa run --once` | Config loaded, State, Registered, One collection cycle complete |

Then leave `oa run` going in terminal 2 for everything below.

## 2b. Auto-detect and states (board: CLI, auto-detect and states)

| State | Run | Compare |
|---|---|---|
| First start, scan running | `oa status` as soon as `oa run` starts | Host panel always on, scan line |
| Suggestions | `oa discover` | Grouped suggestions, confidence, `new` marks |
| Rescan | `oa discover --rescan` | Look again; kinds that could not be scanned come with the fix |
| Dismiss and restore | `oa discover dismiss <id>`, `oa discover --dismissed`, `oa discover restore <id>` | Dismissed list, the item returns |
| Ignore | `oa discover ignore port:5432` | Saved to `discovery.ignore` |
| Scanning off | set `discovery.enabled: false` in `/tmp/orion-play/config.yaml`, then `oa discover` | "Scanning is off" and how to turn it on |
| A kind off | set `discovery.kinds` to a subset | The off kind is named |
| Scan blocked (host) | needs Docker that the agent user cannot read | Message, exact fix, the trade-off |
| Offline | stop Core (terminal 1), wait, `oa status` | "agent offline since …, N suggestions kept" |
| Hand-written config wins | `oa config show` | Known difference: see "Open differences" |

## 3. Credentials (board: CLI, enroll and credentials)

| Screen | Run | Compare |
|---|---|---|
| Token apply | `oa token apply` on a terminal | Masked input |
| Reconfigure | `oa reconfigure` without a token | Says a fresh Add server token is required, with the fix |
| State init | `oa state init` | One line |
| Rejected token (host) | needs a Core with authentication on | Calm error and the recovery command |

## 4. Status, doctor, logs, usage (boards: status, doctor, log viewer, discover and usage)

| Screen | Run | Compare |
|---|---|---|
| Status card | `oa status` | Chip, Host, Monitors bar and table, Reporting, next steps |
| Short | `oa status --short` | One line |
| Watch | `oa status --watch` | Redraws in place, `q` quits, scrollback intact |
| No recent report | stop Core, wait several minutes, `oa status` | "No recent report", queue count, doctor hint |
| Maintenance | `oa maintenance down "kernel upgrade" --for 2h`, `oa status` | Chip, reason, until, `maintenance up` hint |
| Not running | stop `oa run`, `oa status` | "Not running" and `service start` |
| Doctor | `oa doctor` on a terminal | Groups Agent, Core, Discovery, Logs, System in colour; problems expand with Fix. In the sandbox it exits 1 because no service is installed, and `NO_COLOR` shows a plain list |
| Doctor failing | `oa --config /tmp/orion-play/bad.yaml doctor` with a wrong `core_url` | Failed checks with fixes, exit code 1 |
| Logs | `oa logs --file /tmp/orion-play/agent.log --lines 20` | Plain lines |
| Log viewer (host) | `orion-agent logs -f` | Needs log shipping (a `logs.sources` entry and its ingest key), so it cannot start in the sandbox. Full screen; arrows, enter details, `p` pause, `l` level, `/` search, `q` |
| Usage | `oa usage --window 2s --top 5` | Table, storage section |
| Usage watch | `oa usage --watch` | `s` sorts, `q` quits |

## 5. Operate (boards: start, stop, restart, run, maintenance, update)

| Screen | Run | Compare |
|---|---|---|
| Run | `oa run` | Checklist, then the one dim "Running and reporting" line (not on the board) |
| Run once | `oa run --once` | Config loaded, State, Registered, One collection cycle complete |
| Maintenance down | `oa maintenance down` (no arguments) | Short form with duration choices |
| Edge cases | `oa maintenance down "disk swap"`, `oa maintenance down --for 800h` | Reason is kept; the long time is refused with the limit |
| Maintenance up | `oa maintenance up` | "Maintenance off", Core told |
| Start, stop, restart (host) | `orion-agent start`, `stop`, `restart` on an installed service | Checklists, stop confirms |
| Start preflight fails (host) | `orion-agent start` with the state database not writable | Error, fix, exit 1 |
| Service install (host) | `orion-agent service install` | Plain copyable installer line |
| Update (host) | `orion-agent update` | Live checklist with progress bar; release notes first on a terminal |
| Update fails and rolls back (host) | point `--base-url` at a bad mirror | "Rolled back", exit 1 |
| Rollback (host) | `orion-agent update --rollback` | "Rolled back to …" |

## 6. Config, help and errors (boards: config, help and errors)

| Screen | Run | Compare |
|---|---|---|
| Show | `oa config show` | Sources and Settings panels, origin chips |
| Edit | `oa config edit` | Opens your editor, validates, diff, one confirm |
| Validate | `oa config validate` | Pass line, or an error with the code frame |
| Diff | `oa config diff` | Tinted unified diff |
| Help | `oa --help` | START HERE, DAY TO DAY, the rest on one line |
| Help topics | `oa help topics`, `oa help discovery`, `oa help exit-codes` | Styled pages (not on the boards) |
| Unknown command | `oa statsu` | `did you mean "status"?` |
| Bad flag | `oa status --bogus` | An error header and a `fix:` line |
| Needs elevation (host) | `orion-agent start` as a normal user | Exit code 3 and "re-run with sudo" |

## 7. Scripts and output modes (boards: emit and run-job, output modes)

| Screen | Run | Compare |
|---|---|---|
| emit | `oa emit --severity error --event backup.failed "nightly backup failed"` | One plain line. Needs an ingest key from the Console (`--key-file`); in the sandbox it prints "no ingest key" |
| run-job ok | `oa run-job --name nightly -- true` | Output unchanged, one summary (plus a "not shipped: no ingest key" note in the sandbox) |
| run-job fails | `oa run-job --name nightly -- false; echo $?` | Summary, and the job's exit code is passed through (works without a key) |
| Plain | `NO_COLOR=1 oa status`, `oa status \| cat` | Same words, no colour |
| JSON | `oa status --json`, `oa doctor --json` | One document, `schema_version` |
| Exit codes | `echo $?` after the failing commands above | 0 ok, 1 failed, 2 usage, 3 elevation |

## 8. Install script (board: CLI, install script)

Needs a real server and a published release, so it cannot run in the sandbox. Run it on a spare host
once a release exists. Compare: five steps in a panel, the "is connected" card with Doctor, Collecting
and Detected lines, the failed-checksum screen (exit 1), the already-installed screen. Its shell tests
(`make agent-installer-test`) cover the text in the meantime.

## Open differences from the boards

- `config show` follows the Config board; the auto-detect board's "hand-written config wins" screen
  draws separate `discovery` and `monitors` panels. Undecided which to follow.
- A continuous `oa run` prints one dim line after "Registered"; the board prints nothing.
- `help topics` and Glamour-style release notes before `update` are additions, not on the boards.
