# Orion – Project TODO & Roadmap

This document outlines the current and upcoming work for **Orion**, covering the **Core**, **Agent**, and **Frontend**.  
It is intended as a living roadmap that can be used directly in Cursor / GitHub.

---

## 0. Cross-Cutting Foundations (High Priority)

- [x] Define shared **status vocabulary**
  - Health: `up | down | degraded | unknown`
  - Lifecycle: `active | deleted | disabled`
- [x] Define **monitor report contract**
  - What the agent sends
  - What the core persists
- [x] Add structured error codes for agent ↔ core communication
- [ ] Version all public APIs (`/v1`)
- [ ] Add request IDs / trace IDs for debugging
- [ ] Document agent ↔ core responsibilities clearly

---

## 1. Core (Backend)x

### 1.1 Domain & APIs

#### Agents

- [x] Finalize agent registration flow
- [ ] Track agent last-seen timestamp
- [ ] Handle agent reconnect / re-registration safely
- [ ] Support agent maintenance mode (core override)

#### Monitors

- [x] Monitor registration (idempotent)
- [x] Monitor unregistration (soft delete)
- [x] Revive soft-deleted monitors
- [x] Enforce unique monitor name per agent
- [x] Persist lifecycle separately from health

#### Reports & Health

- [x] Store system reports efficiently
- [x] Store monitor reports (JSON payloads)
- [ ] Compute derived health:
  - degraded logic
  - flapping detection
  - stale data detection
- [ ] Track last successful report per monitor

---

### 1.2 Endpoints for Frontend

#### Agents

- [ ] List agents
- [ ] Agent detail endpoint
  - system metrics
  - last seen
  - platform / OS / arch
- [ ] Agent health summary

#### Monitors

- [ ] List monitors per agent
- [ ] Monitor detail endpoint
  - current health
  - recent history
- [ ] Monitor history / timeline endpoint
- [ ] Filter monitors by status

#### Aggregates

- [ ] Overall system health endpoint
- [ ] Degraded / down summary endpoint
- [ ] Incident candidate endpoint

---

### 1.3 Data & Storage

- [ ] Add DB indexes:
  - agent_id
  - monitor_id
  - created_at
- [ ] Retention policy for reports
- [ ] Optional rollups (hourly / daily)
- [ ] Soft-delete cleanup job (optional, later)
- [ ] Migrations strategy

---

### 1.4 Deployment & Infrastructure

- [ ] Dockerfile (multi-stage build)
- [ ] Docker Compose for local development
- [ ] GitHub Actions:
  - build
  - test
  - lint
  - push Docker image
- [ ] Environment-based configuration
- [ ] Health endpoint for core
- [ ] Graceful shutdown handling

---

## 2. Agent

### 2.1 Installation & Distribution

- [ ] `install.sh` script
  - Detect OS (Linux / macOS)
  - Download correct binary
  - Create config & state directories
- [ ] Generate default config on install
- [ ] Dependency checks (pm2, permissions, etc.)
- [ ] Optional uninstall script

---

### 2.2 Agent CLI

- [ ] `orion-agent start`
- [ ] `orion-agent stop`
- [ ] `orion-agent status`
- [ ] `orion-agent restart`
- [ ] `orion-agent maintainance -up`
- [ ] `orion-agent maintainance -down`
- [ ] `orion-agent upgrade`
- [ ] `orion-agent config validate`
- [ ] `orion-agent config diff`

---

### 2.3 Agent as Background Service

- [ ] systemd unit (Linux)
- [ ] launchd plist (macOS)
- [ ] Auto-restart on failure
- [ ] Log rotation
- [ ] Graceful shutdown on signals

---

### 2.4 Agent Routines & Scheduling

- [ ] Ensure per-monitor goroutine isolation
- [ ] Batch reports before sending
- [ ] Configurable batch size
- [ ] Flush batches on shutdown
- [ ] Retry with backoff on network failure
- [ ] Add jitter to intervals
- [ ] Separate system metrics pipeline from monitor pipeline

---

### 2.5 Agent Maintenance Mode

- [ ] Maintenance flag in internal state
- [ ] Pause reporting while in maintenance
- [ ] Core-triggered maintenance override
- [ ] Maintenance reason + optional TTL
- [ ] Exclude maintenance from health calculations

---

### 2.6 Agent Upgrades

- [ ] Version check against core
- [ ] Safe binary replacement
- [ ] Rollback on failed upgrade
- [ ] Upgrade locks to prevent concurrent updates

---

### 2.7 Monitor Coverage (Agent)

Already implemented:

- [x] System metrics
- [x] HTTP healthcheck
- [x] Website monitor
- [x] Internal service (ping + port)
- [x] PM2 monitor

Planned:

- [ ] Postgres monitor
- [ ] Docker container monitor
- [ ] systemd service monitor
- [ ] Redis monitor
- [ ] Disk threshold monitor

---

## 3. Frontend

### 3.1 Foundations

- [ ] Auth (even basic token auth)
- [ ] Environment configuration
- [ ] API client layer
- [ ] Global error handling

---

### 3.2 Core Views

#### Dashboard

- [ ] Overall system health
- [ ] Agents count
- [ ] Monitors count
- [ ] Degraded / down summary

#### Agents

- [ ] Agent list
- [ ] Agent detail page
- [ ] Status and last-seen indicators

#### Monitors

- [ ] Monitor list per agent
- [ ] Status badges (up/down/degraded)
- [ ] Search & filter
- [ ] Monitor detail view
- [ ] Timeline / history graph

---

### 3.3 UX Improvements

- [ ] Real-time updates (polling first)
- [ ] Clear status color system
- [ ] Empty and error states
- [ ] Responsive / mobile-friendly layout

---

### 3.4 Incidents & Maintenance (Later)

- [ ] Incident detection from degraded/down
- [ ] Incident timeline
- [ ] Acknowledge / resolve flow
- [ ] Maintenance window UI
- [ ] Suppress alerts during maintenance

---

## 4. Future / Nice-to-Haves

- [ ] RBAC / multi-tenant support
- [ ] Alerting (email, Slack, webhooks)
- [ ] Public status page
- [ ] Agent auto-discovery
- [ ] Encryption at rest
- [ ] Plugin system for custom monitors

---

## Guiding Principles

- Agent stays **dumb, reliable, boring**
- Core stays **smart and opinionated**
- Frontend stays **clear, fast, honest**
- Prefer reconciliation over mutation
- Prefer soft-delete over hard-delete
- Never block agent execution on frontend needs
