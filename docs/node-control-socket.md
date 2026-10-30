# Node Control Socket

The daemon (`orion-node run`) listens on a local unix socket so operators can inspect and steer it without sudo.

| | Linux | macOS |
|---|---|---|
| Default path | `/run/orion/node.sock` | `/usr/local/var/run/orion/node.sock` |
| Mode / group | `0660`, group `orion` | `0660`, group `orion` or `admin` |

The socket folder must exist and belong to the service account. On Linux systemd creates it (`RuntimeDirectory=orion`). On macOS nothing does at runtime, because the daemon runs as the unprivileged `_orion` and cannot create folders under `/usr/local/var/run`: the installer creates `/usr/local/var/run/orion`, and `orion-node start`, `restart` and `update` (all root) create it for installs that predate that. Until it exists the node still reports to Core, but `status`, `doctor` and `restart` cannot see it; the fix is `sudo orion-node restart`. Custom `--socket` paths are never created or chowned for you.

Override with `--socket` or `ORION_NODE_SOCKET`. The node token is never returned by any endpoint.

| Route | Purpose |
|---|---|
| `GET /v1/status` | Version, pid, uptime, registration, maintenance, monitor count |
| `POST /v1/maintenance` | `{enabled, reason}`; also updates Core |
| `POST /v1/token` | `{token}` replacement; identity preserved, restart to take effect |
| `POST /v1/reload` | Validates the config on disk and rebuilds the collection loop in place; a bad config is rejected and the running one kept |
| `GET /v1/token-check` | Daemon probes Core with its token and returns only the HTTP status (the token never leaves the daemon) |
| `GET /v1/logs` | Tail (up to 4 MiB) of the daemon JSONL log |

CLI behaviour: `status`, `logs`, `maintenance` and `token apply` try the socket first. They fall back to the direct state-file path (with the legacy sudo re-exec) only when the socket is missing or not permitted. The legacy sudo path is removed one release after this ships (plan section 10).

Trust: on Linux, members of group `orion` (and root) can set maintenance mode, apply a replacement token and read the daemon log through the socket. On macOS the group is `_orion` or `admin`, so every local admin has that access.
