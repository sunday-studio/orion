# Agent and CLI: End-to-End Plan

Status: proposed, awaiting "go". 2026-10-17. Owner: new thread `orion:agent` (`apps/agent/`, with deploy changes in `deploy/`).
Related: [product-logs-plan.md](product-logs-plan.md), [backend-hardening-e2e-plan.md](backend-hardening-e2e-plan.md), [decision-log.md](decision-log.md), `docs/design/design-language.md`.

## 1. Goal

Make the Orion agent and CLI something you can install in about two minutes, configure without editing files, and trust to run everywhere on Linux and macOS. It measures what your apps cost without touching them, and it is one of several ways to get product logs in, never a requirement.

Priority order set by the user (D-016): **install and configure experience first**; performance and cleanup second; log features third; dashboards (`top`) are nice-to-have (D-015). Platforms: Linux and macOS only (D-014).

## 2. Principles

1. **Core is the primary path for logs; the agent is optional** (D-018). An app, including an open-source one on a host without an agent, sends OpenTelemetry (OTLP) straight to Core using endpoint and key set in its own config or env. The agent never has to be in that path.
2. **Passive first** (D-017). Resource usage of apps (CPU, memory, disk, network, storage) is measured from the outside. No app connection, key, or instrumentation required.
3. **Standards over custom.** OTLP for logs. No Orion-specific protocol for apps.
4. **Zero-config works.** With no config file the agent reports system metrics and the host appears in Console.
5. **Least privilege.** The daemon runs unprivileged by default.
6. **One look.** CLI styling comes from the same design tokens as the web design system.
7. Every change has a test; commit as you go; independent review before push (AGENTS.md).

## 3. How data reaches Orion

| Level | What the user does | Needs agent? | What they get |
|---|---|---|---|
| 1. Passive | Nothing (agent installed on the host) | Yes | Per-process, per-container and per-service CPU, memory, disk and network usage; system metrics; health monitors |
| 2. No-code logs | Point the agent at log files, a systemd unit, or a container | Yes | App logs mapped to the log envelope, with offsets and redaction, no code change |
| 3. OTLP direct | Set `OTEL_EXPORTER_OTLP_ENDPOINT` and ingest key header in the app | No | Rich product logs and events in Console |
| 4. OTLP via agent (optional, later) | Point the app at `localhost` or a unix socket | Yes | Same as 3, plus key-less apps, local buffering when Core is unreachable |

Logs from level 3 and host metrics from level 1 are linked by `host.name` / `service.instance.id`, so the Console shows them together without routing logs through the agent.

## 4. Scope

### In scope
- Install, enroll, configure, upgrade, uninstall experience (script, CLI, Console "Add server").
- Daemon and CLI cleanup, performance, privilege model, portability.
- Passive app resource metrics and discovery.
- Log shipping from files, journald and containers, with offsets.
- Charm-based CLI UI aligned with the design system.

### Out of scope
- Windows, FreeBSD (D-014).
- Remote command execution, long-term log storage on device.
- Agent as a required OTLP gateway (D-018). The relay is optional and last.
- The `top` dashboard, until after the above (nice-to-have).

## 5. Architecture changes

```
          ┌────────────── orion-agent (CLI) ──────────────┐
          │ cobra + fang + lipgloss; thin client          │
          └──────────────┬────────────────────────────────┘
                         │ unix socket (0660, group orion), JSON
          ┌──────────────▼────────────────────────────────┐
          │ orion-agentd (daemon, user `orion`)            │
          │  collectors   discovery   log shipper          │
          │  control API  hot config reload                │
          │  spool (pure-Go store)  transport (gzip/zstd)  │
          └──────────────┬────────────────────────────────┘
                         ▼ HTTPS, agent token
                       Core
```

- CLI and daemon talk over a **local control socket**, so `status`, `logs`, `doctor`, `maintenance` need no sudo, and config reloads without SIGHUP.
- State store replaced with a pure-Go store (`modernc.org/sqlite` or an append-log file). This removes CGO, so builds are static and cross-compile from one runner.
- Interactive and TUI packages are kept out of the daemon import path to hold startup and memory budgets.

## 6. Work packages

### Phase A: Install and configure experience (priority)

| WP | Work | Done when |
|---|---|---|
| A1 | **Console "Add server" flow** produces one copy-paste command with a one-time enrollment token baked in (token model from backend ORB-009); Console shows "waiting for agent" then "connected" live. Needs backend (enrollment tokens, agent-connected event), design (screen exists in the map; confirm), web | New host appears in Console with data within 2 minutes of pasting the command, no config edits |
| A2 | **Install script v2** (`agent-bootstrap.sh`): detects OS, arch and init system (systemd, launchd); prints a plan; `--dry-run`; verifies SHA256 and signature (ORB-013); creates the `orion` user and dirs with correct modes; installs the service; starts it; runs `doctor`; idempotent; non-interactive via flags and env (cloud-init, Ansible, Docker); `--user` mode for no-root installs; clean `uninstall` | Re-running is a no-op; tampered binary aborts; uninstall leaves nothing behind (tested in containers and a macOS runner) |
| A3 | **Zero-config defaults.** No config file needed: system metrics on, sensible intervals, Core URL and token from the installer or env | `orion-agent run` with only env vars reports metrics |
| A4 | **`setup` wizard** (Charm `huh`/`bubbletea`): confirms Core URL and token, runs discovery (processes, containers, systemd units, listening ports, common log paths), checkbox list of what to monitor, shows config diff, validates, applies without restart. Non-interactive mode from flags/JSON for automation | Wizard output equals the hand-written config for a fixture host |
| A5 | **Config model.** YAML plus `config.d/` drop-ins, env overrides, secrets from file paths, `config show/diff/validate/edit` with schema validation and helpful errors | Invalid config errors name the line and fix |
| A6 | **`doctor` that fixes.** Streaming checklist: DNS, TLS, reach Core, token valid, clock skew, permissions to read configured logs (`adm`, `systemd-journal`, ACLs), disk space, service state, version. Each failure prints the exact command to fix it | Every failing fixture check has a fix message; `--json` stable |
| A7 | **Safe upgrades.** `update` verifies checksum/signature, stages, swaps atomically, health-checks, rolls back on failure; optional auto-update channel | Failed upgrade leaves the old version running (tested) |
| A8 | **Privilege model.** Daemon runs as `orion` with minimal capabilities and systemd sandboxing; sudo re-exec removed in favor of the control socket; elevation only for install, update, service management; clear message when elevation is needed | `status`, `logs`, `doctor`, `maintenance` work as a normal user in the `orion` group |

### Phase B: Cleanup and performance

| WP | Work | Done when |
|---|---|---|
| B1 | State store swap (pure Go) and static builds for linux amd64/arm64/armv7 and darwin amd64/arm64 from a single runner; musl-compatible | `CGO_ENABLED=0` build passes tests; binaries run on Alpine, Debian, RHEL |
| B2 | Agent audit fixes: SIGHUP (ORB-014), registration retry (015), data races (016), retry queue (017), silent monitor death (018), report idempotency key (with backend ORB-019) | `-race` stress tests pass; agent starts with Core down for 5 min and recovers |
| B3 | Transport: gzip/zstd batches, bounded memory, adaptive batching, request contexts for shutdown | Memory flat under backlog replay; shutdown within 2 s |
| B4 | Performance budgets enforced in CI: CLI cold start under 50 ms for `--help`/`version`, `status` under 100 ms; daemon idle under 1% CPU, RSS under 30 MB with 20 monitors | CI benchmark job fails on regression |
| B5 | Command surface regroup: `service` (install, start, stop, restart, uninstall, status), `config`, `token`, `maintenance`, `update`, `doctor`, `setup`, `logs`, `discover`, `emit`, `run-job`, `version`. Stable `--json` schemas everywhere. Keep old names as hidden aliases for one release | `--json` golden tests for every command |
| B6 | Split oversized files; keep under 500 lines | `make code-line-limit` passes |

### Phase C: Charm UI aligned with the design system

| WP | Work | Done when |
|---|---|---|
| C1 | **Theme generated from design tokens.** A small generator reads the design token source (shared with web) and emits a Go theme: palette, status glyph shapes (up, down, degraded, unknown), spacing. Adaptive light/dark, truecolor/256/16 fallback, `NO_COLOR` and non-TTY plain output | Changing a token regenerates CLI colors; CI checks generated file is current |
| C2 | Styled primitives with `lipgloss`: headers, key-value, status chips, tables (`lipgloss/table`), callouts, steps with spinners; `fang` for help and errors on top of cobra | `status`, `version`, `config show` and error output share one look; screenshot tests |
| C3 | Interactive pieces with `bubbletea`/`bubbles`/`huh`: setup wizard, doctor checklist, log viewer (`logs --follow` with filters and level highlighting) | Works over SSH and in tmux; degrades to plain when not a TTY |
| C4 | Accessibility: no color-only meaning (glyph plus text), respects `NO_COLOR`, readable at 80 columns | Snapshot tests at 80 and 120 columns |

### Phase D: Passive app resource metrics and discovery

| WP | Work | Done when |
|---|---|---|
| D1 | **Spike (2 days, report first):** what is reliably measurable per OS. Linux: per-process CPU/RSS/IO (`/proc`), per-systemd-unit and per-container cgroup v2 stats (cpu, memory, io, pids), container network counters via the Docker API and net namespaces, disk usage of paths and Docker volumes. Per-process network bandwidth is hard without eBPF or privileges, so the spike decides: per-container and per-unit network first; per-process only where cheap. macOS: process CPU/memory/IO, limited network. Output: a support matrix doc | Matrix committed; decisions for D2 logged |
| D2 | **Collectors** for per-process, per-systemd-unit, per-container usage (CPU, memory, disk I/O, network where supported) and storage (configured paths, Docker volumes, log sizes), sampled cheaply with caps on cardinality | Metrics appear in Core for a fixture app with no integration |
| D3 | **Discovery** (`discover`, also used by the wizard): systemd units, containers, pm2, listening ports, common log locations; proposes monitors and sources with confidence labels | Fixture host discovery matches expected set |
| D4 | Backend contract for these metrics: new report section(s), `docs/agent-core-contract.md` and OpenAPI updated (with backend thread) | Contract tests pass |

### Phase E: Logs from the device

| WP | Work | Done when |
|---|---|---|
| E1 | **Log shipper** replacing the current rescan-the-file approach (`internal/service_logs.go` re-reads whole file and keeps last 100): persistent offsets per file with rotation/truncation handling, journald and container sources, multi-line joining, edge redaction, bounded queue and spool, backpressure | Rotating file under burst loses nothing and duplicates nothing; tests with 1M lines |
| E2 | Mapping to the product-log envelope (OTel model) and shipping to the Core ingest API with a per-source ingest key; sources configured in YAML or discovered | Entries show in Console Logs page |
| E3 | `emit` (send one entry from a script) and `run-job -- <cmd>` (capture output and exit code, heartbeat plus logs) for cron and scripts | Failed job produces an error entry and a missed-heartbeat alert |
| E4 | Local `logs` command reads from local spool, works offline | Works with Core unreachable |
| E5 | **Optional OTLP relay** (unix socket default, TCP localhost opt-in). Last; only if wanted after real use | Stock OTel SDK sends via relay; buffers when Core is down |

### Phase F: Distribution

| WP | Work | Done when |
|---|---|---|
| F1 | Release pipeline: static builds, `SHA256SUMS`, signature, SBOM, reproducible tags | Install script verifies end to end |
| F2 | Container image (rootless, read-only FS) and Kubernetes DaemonSet manifest/Helm chart with env-only config | Pod reports host and pod metrics on kind in CI |
| F3 | Docs: install guide per platform, troubleshooting from `doctor` messages, config reference generated from the schema | Docs built in CI |

## 7. Dependencies on other threads

| Need | From | Needed by |
|---|---|---|
| One-time enrollment tokens, agent-connected event/endpoint (ORB-009) | backend | A1, A2 |
| Report idempotency (ORB-019) | backend | B2 |
| Product-log ingest contract and ingest keys (L0/L1) | backend | E2 |
| New metrics report sections | backend | D4 |
| "Add server" Console flow, connect-agent live state | web, design | A1 |
| Shared design token source in a machine-readable form | design / web | C1 |

The agent thread works against the contract docs without waiting; it does not message other threads. Needs go through the coordinator.

## 8. Order of work

```
A (install/config) ─┬─ B (cleanup) ─ C (Charm UI) ─ D (passive metrics) ─ E (logs) ─ F (distribution)
                    └─ B1 first: static builds unlock A2 on all targets
```

Start with B1 (static builds) and A8 (control socket), because most of A depends on them. A2, A3, A5, A6 can then proceed in parallel with C1/C2.

## 9. Testing

- **Unit and race:** `-race` on all packages; table-driven tests for config, discovery, redaction.
- **Install matrix in CI:** Debian, Ubuntu, Alpine, RHEL-family containers with systemd, plus a macOS runner with launchd; install, enroll against a test Core, assert data arrives, upgrade, roll back, uninstall.
- **Golden tests:** `--json` output per command; CLI snapshots at 80 and 120 columns, color and `NO_COLOR`.
- **Failure injection:** Core down at boot, Core slow, token revoked, clock skew, disk full, permission denied on log file, log rotation mid-read.
- **Performance:** startup, idle CPU/RSS, ship throughput (target 5k lines/s on a small VM) as CI benchmarks.
- **Security:** no secrets in logs or `ps`, config files `0600`, socket permissions, installer tamper test.
- **End to end with Core:** an app with no integration shows usage; a file-tailed app shows logs; an app using a stock OTel SDK sending directly to Core shows logs with no agent present.

## 10. Rollout

- Old command names stay as hidden aliases for one release; `--json` schemas are versioned.
- Control-socket change ships alongside the old sudo path for one release, then the old path is removed.
- System service remains the default install; `--user` mode documented for laptops.
- No push without the user's go-ahead.

## 11. Decisions made

| # | Decision | Alternative rejected |
|---|---|---|
| G1 | Core is the primary OTLP target; agent optional (D-018) | Agent as required middleman |
| G2 | Passive metrics need no app integration (D-017) | Require instrumentation |
| G3 | System service by default, unprivileged `orion` user, `--user` mode available | Root daemon (today) |
| G4 | CLI to daemon via local control socket | sudo re-exec per command |
| G5 | Pure-Go state store, static builds | Keep CGO SQLite |
| G6 | Keep cobra; add fang, lipgloss, bubbletea, huh | Rewrite CLI framework |
| G7 | Theme generated from shared design tokens | Hand-copied colors |
| G8 | Per-process bandwidth only if cheap; spike decides (D1) | Promise it up front |
| G9 | OTLP relay is optional and last | Build it first |

## 12. Risks

| Risk | Mitigation |
|---|---|
| Per-process network usage is not portable or needs privilege | Spike first; ship per-container and per-unit; document the matrix honestly |
| macOS limits (no cgroups, restricted APIs) | Reduced metric set on macOS, stated in the matrix |
| Pure-Go SQLite behaves differently under load | Benchmark before swapping; fall back to append-log store |
| Installer is the product's front door and a supply-chain risk | Verify signatures, tamper tests, dry-run, clear plan output |
| Scope is large | Phase A and B1 ship value alone; later phases are separable tickets |
| Cardinality blowup from per-process metrics | Caps, allowlists from discovery, top-N only |

## 13. Tickets

Tickets: AG-1 Static builds and state store (B1), AG-2 Control socket and privilege model (A8), AG-3 Install script v2 (A2), AG-4 Enrollment and Add-server flow, agent side (A1), AG-5 Zero-config and config model (A3, A5), AG-6 Setup wizard and discovery (A4, D3), AG-7 Doctor (A6), AG-8 Safe upgrades (A7), AG-9 Audit fixes (B2), AG-10 Transport and budgets (B3, B4), AG-11 Command regroup and JSON schemas (B5, B6), AG-12 Charm theme and primitives (C1, C2, C4), AG-13 Interactive UI (C3), AG-14 Passive metrics spike (D1), AG-15 Passive collectors and contract (D2, D4), AG-16 Log shipper (E1, E2, E4), AG-17 emit and run-job (E3), AG-18 Release pipeline (F1), AG-19 Container and DaemonSet (F2), AG-20 Docs (F3), AG-21 OTLP relay (E5, last, optional).
