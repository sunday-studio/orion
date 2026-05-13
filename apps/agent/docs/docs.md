# Orion Agent — Phase 1 Design

## Overview

The **Orion Agent** is a lightweight Go daemon installed on servers (Linux/macOS).  
Its job is to:

- Collect system information (CPU, memory, disk, uptime, etc.)
- Manage sub-processes defined in its config
- Periodically send system data to **Orion Core**
- Run as a background service (systemd or launchd)
- Be fast and minimal while keeping user config in YAML and Agent-owned runtime state in local SQLite

---

## Responsibilities

1. **Configuration**

   - Read `/etc/orion/config.yaml` (or local `config.yaml` if running manually)
   - Keep identity, token, monitor ids, and maintenance state in `state.db`
   - Contain fields for:
     ```yaml
     core_url: "https://orion-core.example.com"
     interval: "60s"
     monitors:
       - name: "homepage"
         type: "http-healthcheck"
         interval: "30s"
         http:
           url: "https://example.com"
           timeout: "5s"
           expected_status: 200
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
     POST /v1/agents/{id}/report
     Content-Type: application/json
     Authorization: Bearer <token>
     ```

4. **Service Management**
   - Commands:
     - `orion start` — start agent loop
     - `orion stop` — stop agent
     - `orion status` — check if running
     - `orion-agent run -once` — manually collect and send once

---

## Folder Structure
