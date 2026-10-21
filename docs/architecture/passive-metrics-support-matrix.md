# Passive App Resource Metrics: Support Matrix

Spike D1 for [agent-cli-plan.md](../plans/agent-cli-plan.md). Status: findings, decisions for D2 at the end. Date: 2026-10-17.

Goal (D-017): measure what an app costs (CPU, memory, disk, network, storage) from the outside, with no app change, no key, and no elevated daemon privilege (D-016, plan principle 5).

## How this was measured

- macOS (arm64, unprivileged user, 1,221 processes): `gopsutil/v4` per-process calls, `nettop -P -L 1`.
- Linux (OrbStack kernel, cgroup v2, Alpine 3.20 container and a `--pid=host` container): read `/sys/fs/cgroup` and `/proc` directly.
- Not measured here: cgroup v1 hosts, RHEL with SELinux, kernels with `hidepid=2`. Those are listed as "verify in the install matrix".

## Linux

| Signal | Source | Needs | Cost | Verdict |
|---|---|---|---|---|
| Per-process CPU, RSS, state | `/proc/<pid>/stat`, `statm` | world-readable by default | O(processes) | **Ship.** Top-N only |
| Per-process read/write bytes | `/proc/<pid>/io` | same uid or `CAP_SYS_PTRACE` | O(processes) | **Best effort.** Works for the daemon's own user; other users' processes return permission denied. Do not grant ptrace |
| Per-unit and per-container CPU | `cpu.stat` `usage_usec` | world-readable, cgroup v2 | O(cgroups) | **Ship.** Measured: file present at the cgroup root and in containers |
| Per-unit and per-container memory | `memory.current`, `memory.stat` | world-readable | O(cgroups) | **Ship** (`memory.current` measured: 3.5 MB for an idle shell container) |
| Per-unit and per-container disk I/O | `io.stat` (`rbytes`, `wbytes` per device) | world-readable | O(cgroups) | **Ship.** Measured |
| Process count | `pids.current` | world-readable | O(cgroups) | **Ship** |
| Per-process network | none without eBPF, `CAP_NET_ADMIN`, or libpcap | privilege | high | **Do not ship** (G8). Listed as not supported |
| Per-unit network | no cgroup counter exists | eBPF | high | **Do not ship** |
| Per-container network | Docker API `stats` (`networks`), or `/proc/<init-pid>/net/dev` | docker socket (group `docker`, root-equivalent) or ptrace on the container init | O(containers) | **Opt-in** collector. The installer no longer adds the `orion` user to `docker` by default (`--docker-access` opts in); `doctor` explains the trade-off |
| Host network | `/proc/net/dev` | world-readable | trivial | Already collected at host level |
| Path and volume disk usage | `statfs`, capped directory walk; Docker volumes via `system/df` | read access to the path | walk cost is bounded by caps | **Ship** for configured paths; volumes opt-in with Docker |
| cgroup v1 hosts | `cpuacct`, `memory.usage_in_bytes`, `blkio` | world-readable | | **Fallback only for CPU and memory**; I/O reported as unavailable |

Mapping an app to a cgroup: systemd units live at `system.slice/<name>.service`; Docker at `system.slice/docker-<id>.scope` or `docker/<id>`; any PID maps through `/proc/<pid>/cgroup`. This covers pm2 and plain processes through the user slice they run in.

## macOS

| Signal | Source | Needs | Verdict |
|---|---|---|---|
| Per-process CPU, RSS | `gopsutil` (libproc via purego, no cgo) | none | **Ship.** Measured 1,221 of 1,221 processes |
| Per-process disk I/O | `proc_pid_rusage` | none for own-user processes | **Best effort.** Measured 994 of 1,221 (81%) readable unprivileged |
| Per-process network | `nettop -P -L 1 -J bytes_in,bytes_out` | none | **Ship as cumulative counters**, sampled by delta. Measured: works unprivileged, 63 rows. Output parsing is brittle; isolate behind one function and tolerate failure |
| Per-service (launchd) | `launchctl list` label to PID, then per-process | none | **Ship** as sum of the service's PIDs |
| Containers (Docker Desktop, OrbStack) | Docker API | socket access | **Opt-in**; stats are VM-level |
| cgroups | n/a | | Not applicable |

## Cardinality and cost rules (apply to every collector)

1. Sample at the report interval (default 60 s), never faster. CPU is a delta between two samples.
2. Report **top N** processes by CPU and by RSS (default N = 10 each) plus every process or unit that matches an allowlist from discovery. Hard cap of 200 series per host.
3. Containers and units: report all up to the cap, then top N by CPU.
4. Never report command lines or environment (secrets). Report executable name, unit or container name.
5. Failure of one source (permission denied, parse error) drops that series and records a reason in the report's `collection_notes`; it never fails the report.

## Decisions for D2

| # | Decision |
|---|---|
| P1 | Linux collectors read cgroup v2 first (units, containers, user slices). cgroup v1 supports CPU and memory only. |
| P2 | Per-process list uses `/proc` and `gopsutil`; I/O is best effort. No `CAP_SYS_PTRACE`, no root. |
| P3 | Per-process and per-unit network bandwidth is **not supported** on Linux. Per-container network is an opt-in Docker collector. macOS gets per-process network through `nettop`. |
| P4 | Disk: configured paths (statfs plus a capped walk, default 2 s budget) and Docker volumes when Docker is enabled. |
| P5 | New report section `app_resources` (see the contract update in `agent-core-contract.md`); series carry `kind` (process, unit, container, path), `name`, and a stable `key`. |
| P7 | `agent-install.sh` stops adding the service user to the `docker` group unless `--docker-access` is passed (behaviour change for fresh installs; existing memberships are untouched). |
| P6 | The systemd unit adds no capabilities for this feature. Reading other users' process I/O is documented as unsupported rather than solved with privilege. |

## Honest gaps

- Per-process network on Linux is a real product gap versus tools with eBPF agents. If it becomes a priority it needs its own decision on privilege (`CAP_BPF`, `CAP_PERFMON`) and a kernel-version matrix.
- Docker-based signals require group `docker`, which is root-equivalent. The default install keeps the daemon out of that group.
- Verify in the install matrix: cgroup v1 (older RHEL), SELinux denials on `/sys/fs/cgroup`, `hidepid` mounts, rootless Docker socket paths.

## Update 2026-10-20: Docker and storage suggestions

- **Docker collector (implemented, off unless the socket is readable):** with access to `/var/run/docker.sock` (`app_resources.docker_socket` to override, `app_resources.docker: false` to never use it) the agent names cgroup containers and adds per-container network rates; on hosts without cgroups (macOS) containers come from the API with memory and network. Without access, containers stay visible by short id and one note says why. The installer still does not grant Docker access unless `--docker-access` is passed.
- **Storage suggestions (implemented in discovery):** `discover` proposes tracking the size of directories an app uses: those its systemd unit declares (`WorkingDirectory`, `StateDirectory`, `LogsDirectory`, `CacheDirectory`, only under `/var/lib`, `/var/log`, `/var/cache`, `/srv`, `/opt`, `/data`, `/home`) and well-known data directories for running apps (PostgreSQL, MySQL, Redis, nginx logs, ...). Accepting one adds it to `app_resources.paths`. A directory the agent cannot read is listed as low confidence with the reason.
- **Docker storage (implemented, same permission as above):** the writable layer of each running container (`/containers/json?size=true`), each volume, and the total for images and build cache (`/system/df`). Docker computes these by walking disk, so they are read at most every five minutes and reused in between. `orion-agent usage` lists them in a Storage panel; at most 50 volumes are reported.
- **Still not supported:** per-process and per-unit network on Linux (needs eBPF or `CAP_NET_ADMIN`; see P3).
