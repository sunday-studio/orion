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
# terminals 2 and 3: the agent command, a fresh (not enrolled) host, and the Core address
oa()  { ORION_LOG_PATH=/tmp/orion-play/agent.log /tmp/orion-play/oa --config /tmp/orion-play/config.yaml --state /tmp/orion-play/state.db --socket /tmp/orion-play/a.sock "$@"; }
oaf() { ORION_LOG_PATH=/tmp/orion-play/fresh.log /tmp/orion-play/oa --config /tmp/orion-play/fresh.yaml --state /tmp/orion-play/fresh.db --socket /tmp/orion-play/fresh.sock "$@"; }
CORE=http://127.0.0.1:18990
```

Reset everything: stop the agent, `rm -rf /tmp/orion-play/{config.yaml,state.db,fresh.*,data}`, restart Core.

## 1. Setup (board: CLI, setup)

| Screen | Run | Compare |
|---|---|---|
| Connect | `oaf setup` | Step 1 of 3, Core address with reachable check, masked one-time token, key hints |
| Choose | continue the same flow | Grouped panels (Services, Containers, Ports, Disks), `already monitored` rows, confidence column |
| Scan settings | open it from Choose | Kinds on/off, report toggle |
| Review | continue | Config diff with tinted lines, one confirm |
| Applied | confirm | Result card, next step |
| Non-interactive | `oaf setup --core-url $CORE --accept-discovered --yes --init-state` | Checklist: Scanned, Accepted, Skipped, Config written |
| Accessible | `ACCESSIBLE=1 oaf setup --dry-run` | Plain line prompts, no animation |

## 2. Auto-detect and states (board: CLI, auto-detect and states)

First start the agent in terminal 2: `oa setup --core-url $CORE --accept-discovered --yes --init-state && oa enroll --core-url $CORE --token play && oa run`.

| State | Run | Compare |
|---|---|---|
| First start, scan running | `oa status` right after `oa run` starts | Host panel always on, "scan running" line |
| Suggestions | `oa discover` | Grouped suggestions, confidence, `new` marks |
| Rescan | `oa discover --rescan` | Look again, kinds that could not be scanned with the fix |
| Dismiss and restore | `oa discover dismiss <id>`, `oa discover --dismissed`, `oa discover restore <id>` | Dismissed list, items return |
| Ignore | `oa discover ignore port:5432` | Saved to `discovery.ignore` |
| Scanning off | `oa discover` after setting `discovery.enabled: false` in the config | "Scanning is off" and how to turn it on |
| A kind off | set `discovery.kinds` to a subset | The off kind is named |
| Scan blocked (host) | needs Docker not readable by the agent user | Message, exact fix, the trade-off |
| Offline | stop Core (terminal 1), wait, `oa status` | "agent offline since …, N suggestions kept" |
| Hand-written config wins | `oa config show` | Known difference: see "Open differences" |

## 3. Enroll and credentials (board: CLI, enroll and credentials)

| Screen | Run | Compare |
|---|---|---|
| Enroll | `oaf enroll --core-url $CORE --token play` | Reachable, Token accepted, Enrolled as … |
| Enroll again | run it again | "Already enrolled", nothing happens |
| Wrong token | `oaf enroll --core-url $CORE --token` with an empty or odd value | Calm error with the fix |
| token apply | `oa token apply` | Masked input |
| reconfigure, state init | `oa reconfigure`, `oa state init` | Checklist, one line |

## 4. Status, doctor, logs, usage (boards: status, doctor, log viewer, discover and usage)

| Screen | Run | Compare |
|---|---|---|
| Status card | `oa status` | Chip, Host, Monitors bar and table, Reporting, next steps |
| Short | `oa status --short` | One line |
| Watch | `oa status --watch` | Redraws in place, `q` quits, scrollback intact |
| Not enrolled | `oaf status` | Chip "Not enrolled" and `orion-agent setup` |
| Waiting | `oaf enroll ...` then `oaf status` before the daemon runs | "Waiting, first report due within 30 s" |
| No recent report | stop Core, wait several minutes, `oa status` | "No recent report", queue count, doctor hint |
| Maintenance | `oa maintenance down "kernel upgrade" --for 2h`, `oa status` | Chip, reason, until, `maintenance up` hint |
| Not running | stop `oa run`, `oa status` | "Not running" and `service start` |
| Doctor | `oa doctor` | Groups Agent, Core, Discovery, Logs, System; problems expand with Fix |
| Doctor failing | `oa --config /tmp/orion-play/bad.yaml doctor` with a wrong `core_url` | Failed checks with fixes, exit code 1 |
| Logs | `oa logs --lines 20` | Plain lines |
| Log viewer | `oa logs -f` | Full screen; arrows, enter details, `p` pause, `l` level, `/` search, `q` |
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
| Help topics | `oa help topics`, `oa help discovery` | Styled pages (not on the boards) |
| Unknown command | `oa statsu` | "Did you mean status" |
| Bad flag | `oa status --bogus` | Three-line error |
| Needs elevation (host) | `orion-agent start` as a normal user | Exit code 3 and "re-run with sudo" |

## 7. Scripts and output modes (boards: emit and run-job, output modes)

| Screen | Run | Compare |
|---|---|---|
| emit | `oa emit --severity error --event backup.failed "nightly backup failed"` | One plain line |
| run-job ok | `oa run-job --name nightly -- true` | Output unchanged, one summary |
| run-job fails | `oa run-job --name nightly -- false; echo $?` | Summary, exit code passed through |
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
