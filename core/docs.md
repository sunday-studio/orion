# 🛰️ Orion Core Server — Design Plan

## Overview

The **Orion Core Server** is the central hub that receives, authenticates, and stores telemetry reports from multiple Orion Agents running on external machines.

Agents periodically collect data (system config, status, metrics, etc.) and send JSON payloads to the Core Server at defined intervals.  
The Core acts as the persistent datastore and API backend for the UI.

This service focuses on:
- Secure agent registration
- Authentication via permanent tokens
- Reliable storage of incoming reports
- Future expandability for analytics and dashboards

---

## Tech Stack

- **Language:** Go (Golang)
- **Framework:** Gin Web Framework
- **Database:** SQLite (via GORM ORM)
- **Auth:** Static token-based (non-expiring)
- **JSON-based APIs**
- **Single binary deployment**

---

## Core Concepts

### Agents
Each Orion Agent runs on a server or local machine and registers once with the Core.  
The registration returns:
- a unique **Agent ID** (used in URLs)
- a **permanent authentication token**

Agents use this token for all subsequent API calls.

### Reports
Each Agent periodically sends a **report** to the Core Server.  
Reports are stored as raw JSON payloads in the database, linked to the reporting agent.

---

## API Design

### 1. `POST /register`
Registers a new agent.

**Request Body:**
- UUID (unique identifier from the agent)
- Name (hostname or friendly label)
- OS (linux, mac, etc.)
- Arch (architecture)

**Response:**
- Agent ID
- Permanent Token

**Behavior:**
- If agent with same UUID exists, return existing token.
- Otherwise, create a new record.

---

### 2. `POST /report/:agent_id`
Receives periodic status data from a registered agent.

**Headers:**
- `Authorization: Bearer <token>`

**Request Body:**
- JSON payload (system metrics, service status, configs, etc.)

**Behavior:**
- Validate token belongs to the given agent.
- Persist payload to database.
- Update agent’s `last_seen` timestamp.

**Response:**
- Success message and timestamp.

---

## Database Schema

### Agents Table
Stores metadata and permanent authentication tokens for all registered agents.

| Column     | Type     | Description                            |
|-------------|----------|----------------------------------------|
| id          | integer  | Primary key                            |
| uuid        | string   | Unique identifier from the agent       |
| name        | string   | Agent name or hostname                 |
| os          | string   | Operating system                       |
| arch        | string   | System architecture                    |
| token       | string   | Permanent token for authentication     |
| created_at  | datetime | Registration timestamp                 |
| last_seen   | datetime | Last report timestamp                  |

### Reports Table
Stores all reports sent by agents.

| Column     | Type     | Description                            |
|-------------|----------|----------------------------------------|
| id          | integer  | Primary key                            |
| agent_id    | integer  | Foreign key → Agents.id                |
| payload     | text     | Raw JSON report from agent             |
| created_at  | datetime | Time the report was received           |

---

## System Flow

### Agent Registration
1. Agent binary starts for the first time.
2. Sends a registration request to `/register` with system info.
3. Core creates (or reuses) an Agent record and returns:
   - Agent ID
   - Permanent token
4. Agent saves this data locally in its config.

### Periodic Reporting
1. Agent collects metrics every interval (e.g., 30s).
2. Sends JSON payload to `/report/:agent_id` with Authorization header.
3. Core validates token, saves report, and updates last_seen timestamp.
4. UI or other systems can later query the Core for status or analytics.

---

## Error Handling

- **401 Unauthorized:** Missing or invalid token.
- **400 Bad Request:** Malformed request payload.
- **404 Not Found:** Agent ID does not exist or token mismatch.
- **500 Internal Server Error:** DB or server issue.

---

## Logging & Observability

- Log all registration and report requests (with timestamps).
- Track total reports received and last_seen timestamps.
- Simple structured logging (JSON or plain).

---

## Future Enhancements

- Web dashboard to visualize agent health and metrics.
- Token revocation and rotation system.
- Support for message queues (e.g., NATS or Kafka) for large-scale deployments.
- API pagination and filtering.
- Compressed or encrypted payloads.
- Alerting system for stale agents.

---

## Folder Structure Plan

orion-core/
├── cmd/orion-core/
│ └── main.go # Server entrypoint
├── internal/
│ ├── api/ # HTTP route handlers
│ │ ├── routes.go
│ │ ├── auth.go
│ │ └── report.go
│ ├── db/ # SQLite setup and models
│ │ ├── db.go
│ │ └── models.go
│ ├── service/ # Core business logic
│ │ ├── auth_service.go
│ │ ├── agent_service.go
│ │ └── report_service.go
│ └── utils/ # Helpers (token generation, responses)
│ ├── token.go
│ └── response.go
└── go.mod


---

## Summary

- The **Orion Core Server** is a lightweight control plane to receive and store data from remote agents.
- Uses **SQLite** for simplicity and speed.
- Provides **two main endpoints**:
  - `/register` for one-time agent registration
  - `/report/:agent_id` for ongoing telemetry
- Each agent has a **non-expiring token** for secure communication.
- The architecture is modular, easy to extend with analytics, dashboards, or remote commands later.



