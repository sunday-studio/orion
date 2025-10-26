
# Orion System Design

## Overview

**Orion** is a two-part system designed to monitor and broadcast system information from distributed servers to a central core service.  
It consists of:

1. **Orion Agent** – a lightweight Go binary installed on individual machines (Linux/macOS).
2. **Orion Core** – a central Go-based server with a simple frontend UI that visualizes data received from agents.

Each agent periodically collects system information (config, CPU/memory/disk usage, running processes, uptime, etc.) and transmits it securely to Orion Core. The Core persists this data in a local SQLite database and serves it to the UI.

---

## Architecture

      ┌────────────────────┐
      │   Orion Frontend   │
      │ (React/Next.js UI) │
      └────────┬───────────┘
               │ REST / WebSocket
               ▼
      ┌────────────────────┐
      │    Orion Core      │
      │  Go + SQLite DB    │
      └────────┬───────────┘
               ▲
   Periodic    │
   JSON POSTs  │
               ▼
      ┌────────────────────┐
      │   Orion Agent      │
      │  Go binary daemon  │
      │  Runs on servers   │
      └────────────────────┘




---

## Components

### 1. Orion Agent

A lightweight daemon written in Go.

**Responsibilities**
- Install and configure via a single binary (`orion-agent`).
- Load configuration file (`/etc/orion/config.yaml`).
- Collect:
  - Hostname
  - OS info
  - CPU/memory/disk stats
  - Running processes / uptime
  - Custom metadata (from config)
- Manage sub-processes or hooks defined in the config.
- Broadcast system info to the Orion Core at defined intervals (default: every 60s).
- Use HTTPS and token-based auth for communication.

**Key Commands**
- `orion install` — setup systemd service or macOS launch daemon.
- `orion start` — start the agent.
- `orion stop` — stop the agent.
- `orion status` — check if agent is running.
- `orion send` — manually trigger sync.

**UserConfig Example**
```yaml
core_url: "https://orion-core.example.com"
interval: 60s
token: "abcd1234"
subprocesses:
  - name: "nginx_monitor"
    cmd: "/usr/bin/nginx -t"
  - name: "backup_check"
    cmd: "/usr/local/bin/backup-status"
