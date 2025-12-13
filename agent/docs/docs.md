# Orion Agent — Phase 1 Design

## Overview

The **Orion Agent** is a lightweight Go daemon installed on servers (Linux/macOS).  
Its job is to:

- Collect system information (CPU, memory, disk, uptime, etc.)
- Manage sub-processes defined in its config
- Periodically send system data to **Orion Core**
- Run as a background service (systemd or launchd)
- Be fast, minimal, and SQLite-free (agent doesn’t persist anything long term)

---

## Responsibilities

1. **Configuration**

   - Read `/etc/orion/config.yaml` (or local `config.yaml` if running manually)
   - Contain fields for:
     ```yaml
     core_url: "https://orion-core.example.com"
     token: "abcd1234"
     interval: "60s"
     subprocesses:
       - name: "nginx_monitor"
         cmd: "/usr/bin/nginx -t"
     ```

2. **Collection**

   - Gather:
     - Hostname
     - OS
     - CPU usage %
     - Memory usage %
     - Disk usage %
     - Uptime
   - Wrap all in a JSON payload

3. **Broadcast**

   - Every `interval`, POST data to Core endpoint:
     ```
     POST /api/agents/{id}/metrics
     Content-Type: application/json
     Authorization: Bearer <token>
     ```

4. **Service Management**
   - Commands:
     - `orion start` — start agent loop
     - `orion stop` — stop agent
     - `orion status` — check if running
     - `orion send` — manually send now

---

## Folder Structure
