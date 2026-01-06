# Agent ↔ Core Contract

This document defines the responsibilities and API contracts between the **Orion Agent** and **Orion Core** services.

## Guiding Principles

- **Agent stays dumb, reliable, boring**: The agent focuses on data collection and transmission. It does not make decisions about health states or business logic.
- **Core stays smart and opinionated**: The core computes derived health, manages state, and makes decisions.
- **Prefer reconciliation over mutation**: Agents declare their state; core reconciles and persists.
- **Never block agent execution on frontend needs**: Agent operations should never be delayed by frontend requirements.

---

## Agent Responsibilities

The agent is responsible for:

1. **System Metrics Collection**

   - Collect CPU, memory, disk statistics
   - Collect uptime information
   - Collect system metadata (OS, arch, platform, kernel version)
   - Optionally collect geo-location data

2. **Monitor Collection**

   - Execute configured monitors (HTTP, website, PM2, internal services, etc.)
   - Collect monitor-specific metrics
   - Determine monitor health status (up/down) based on checks

3. **Registration**

   - Register itself with core on first run
   - Register/unregister monitors as configured
   - Handle re-registration gracefully (idempotent)

4. **Periodic Reporting**

   - Send system reports at configured intervals
   - Send monitor reports when monitors are checked
   - Handle network failures gracefully (retry with backoff)

5. **State Management**

   - Maintain minimal internal state (agent ID, token, monitor IDs)
   - Store state in `state.yaml` for persistence across restarts
   - Do NOT persist report data locally

6. **Error Handling**
   - Log errors but continue operation
   - Retry failed requests with exponential backoff
   - Never crash due to network issues

---

## Core Responsibilities

The core is responsible for:

1. **Agent Management**

   - Register new agents or return existing agent information
   - Track agent last-seen timestamps
   - Manage agent lifecycle (active, disabled, deleted)
   - Support agent maintenance mode

2. **Monitor Management**

   - Register monitors (idempotent)
   - Unregister monitors (soft delete)
   - Revive soft-deleted monitors
   - Enforce unique monitor names per agent

3. **Report Storage**

   - Store agent system reports efficiently
   - Store monitor reports with health status
   - Track last successful report per monitor

4. **Health Computation**

   - Compute derived health states (degraded, flapping)
   - Detect stale data (no reports in X minutes)
   - Aggregate health across agents and monitors

5. **Data Persistence**

   - Store all data in SQLite database
   - Maintain indexes for efficient queries
   - Implement retention policies

6. **API Provision**
   - Provide RESTful APIs for frontend
   - Version all public APIs (`/v1`)
   - Include request IDs for tracing

---

## API Contracts

All API endpoints are versioned under `/v1` (except `/health`).

### Authentication

Protected endpoints require:

- **Header**: `Authorization: Bearer <token>`
- **Token**: Permanent authentication token returned during agent registration
- **Validation**: Token must match the agent ID in the request path

### Request IDs

All requests include:

- **Header**: `X-Request-ID` (optional, generated if not provided)
- **Response Header**: `X-Request-ID` (echoed back)
- **Purpose**: Enable request tracing across services

---

## Endpoints

### 1. Agent Registration

**Endpoint**: `POST /v1/register`

**Description**: Register a new agent or retrieve existing agent information.

**Request Body**:

```json
{
  "machine_id": "unique-machine-identifier",
  "name": "hostname-or-friendly-label",
  "os": "linux|mac|windows",
  "arch": "amd64|arm64|etc"
}
```

**Response** (200 OK):

```json
{
  "success": true,
  "message": "Agent registered successfully",
  "data": {
    "agent_id": "agent-<uuid>",
    "token": "permanent-authentication-token"
  }
}
```

**Behavior**:

- If agent with same `machine_id` exists, return existing `agent_id` and `token`
- Otherwise, create new agent record and generate new token
- Idempotent operation

**Error Responses**:

- `400 Bad Request`: Missing required fields
- `500 Internal Server Error`: Database or server error

---

### 2. Monitor Registration

**Endpoint**: `POST /v1/agents/:agent_id/register-monitor`

**Description**: Register a monitor for an agent (idempotent).

**Headers**:

- `Authorization: Bearer <token>`

**Request Body**:

```json
{
  "agent_id": "agent-<uuid>",
  "name": "monitor-name",
  "description": "Monitor description",
  "type": "http|website|pm2|internal_service|...",
  "last_checked": "2024-01-01T12:00:00Z"
}
```

**Response** (200 OK):

```json
{
  "success": true,
  "message": "Monitor registered successfully",
  "data": {
    "monitor_id": "monitor-<uuid>"
  }
}
```

**Behavior**:

- If monitor with same `name` exists for agent and is active, return error
- If monitor with same `name` exists but is deleted, revive it (set lifecycle to "active")
- Otherwise, create new monitor
- Enforces unique monitor names per agent

**Error Responses**:

- `400 Bad Request`: Missing required fields
- `401 Unauthorized`: Invalid or missing token
- `409 Conflict`: Monitor with same name already exists (and is active)
- `500 Internal Server Error`: Database or server error

---

### 3. Monitor Unregistration

**Endpoint**: `POST /v1/agents/:agent_id/unregister-monitor`

**Description**: Soft-delete a monitor (sets lifecycle to "deleted").

**Headers**:

- `Authorization: Bearer <token>`

**Request Body**:

```json
{
  "agent_id": "agent-<uuid>",
  "monitor_id": "monitor-<uuid>"
}
```

**Response** (200 OK):

```json
{
  "success": true,
  "message": "Monitor unregistered successfully",
  "data": {
    "success": true
  }
}
```

**Behavior**:

- Sets monitor `lifecycle` to "deleted"
- Sets monitor `health` to "unknown"
- Sets `deleted_at` timestamp
- Monitor can be revived by re-registering with same name

**Error Responses**:

- `400 Bad Request`: Missing required fields
- `401 Unauthorized`: Invalid or missing token
- `404 Not Found`: Monitor not found
- `500 Internal Server Error`: Database or server error

---

### 4. Agent Report

**Endpoint**: `POST /v1/agents/:agent_id/report`

**Description**: Receive periodic system metrics from an agent.

**Headers**:

- `Authorization: Bearer <token>`
- `Content-Type: application/json`

**Request Body**:

```json
{
  "uptime_seconds": 123456,
  "timestamp": "2024-01-01T12:00:00Z",
  "cpu": {
    "cores": 4,
    "usage_percent": 45.2,
    "load_1": 1.2,
    "load_5": 1.5,
    "load_15": 1.3
  },
  "memory": {
    "total_bytes": 8589934592,
    "used_bytes": 4294967296,
    "free_bytes": 4294967296,
    "available_bytes": 4294967296,
    "used_percent": 50.0
  },
  "disk": {
    "total_bytes": 107374182400,
    "used_bytes": 53687091200,
    "free_bytes": 53687091200,
    "used_percent": 50.0
  },
  "location": {
    "ip": "192.168.1.1",
    "hostname": "example.com",
    "city": "San Francisco",
    "region": "CA",
    "country": "US",
    "loc": "37.7749,-122.4194",
    "org": "AS12345 Example Org",
    "postal": "94102",
    "timezone": "America/Los_Angeles"
  }
}
```

**Response** (200 OK):

```json
{
  "success": true,
  "message": "Report received successfully",
  "data": {
    "message": "Report received successfully",
    "timestamp": "2024-01-01T12:00:00Z",
    "report_id": "agent_report-<uuid>",
    "type": "agent"
  }
}
```

**Behavior**:

- Validates token matches agent ID
- Stores report in database
- Updates agent `last_seen` timestamp
- Returns report ID

**Error Responses**:

- `400 Bad Request`: Invalid payload or missing required fields
- `401 Unauthorized`: Invalid or missing token
- `500 Internal Server Error`: Database or server error

---

### 5. Monitor Report

**Endpoint**: `POST /v1/agents/:agent_id/:monitor_id/report`

**Description**: Receive health check results from a monitor.

**Headers**:

- `Authorization: Bearer <token>`
- `Content-Type: application/json`

**Request Body**:

```json
{
  "timestamp": "2024-01-01T12:00:00Z",
  "health": "up",
  "metrics": {
    "response_time_ms": 123,
    "status_code": 200,
    "additional_data": {}
  },
  "error": null
}
```

**For down monitors**:

```json
{
  "timestamp": "2024-01-01T12:00:00Z",
  "health": "down",
  "metrics": {},
  "error": {
    "message": "Connection timeout",
    "code": "TIMEOUT"
  }
}
```

**Response** (200 OK):

```json
{
  "success": true,
  "message": "Monitor report received successfully"
}
```

**Behavior**:

- Validates token matches agent ID
- Validates monitor ID exists and belongs to agent
- Stores report with health status
- Updates monitor health state
- Tracks last successful report timestamp

**Error Responses**:

- `400 Bad Request`: Invalid payload or missing required fields
- `401 Unauthorized`: Invalid or missing token
- `404 Not Found`: Monitor not found
- `500 Internal Server Error`: Database or server error

---

## Data Models

### Agent

```go
type Agent struct {
    ID            string
    MachineId     string  // Unique per machine
    Name          string
    OS            string
    Platform      string
    KernelVersion string
    Arch          string
    Token         string  // Permanent auth token
    CreatedAt     time.Time
    DeletedAt     time.Time
    LastSeen      time.Time
    Location      GeoLocation
}
```

### Monitor

```go
type Monitor struct {
    ID          string
    Description *string
    Type        string  // http, website, pm2, internal_service, etc.
    Name        string  // Unique per agent
    AgentID     string
    Lifecycle   string  // active | disabled | deleted
    Health      string  // up | down | degraded | unknown
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   time.Time
}
```

### Agent Report

```go
type AgentReport struct {
    ID            string
    AgentID       string
    CreatedAt     time.Time
    UptimeSeconds uint64
    Timestamp     string
    CPU           CPUStats
    Memory        MemoryStats
    Disk          DiskStats
    Location      GeoLocation
}
```

### Monitor Report

```go
type MonitorReport struct {
    ID          string
    MonitorID   string
    Payload     string  // JSON string of metrics/error
    CollectedAt string
    Health      string  // up | down
    CreatedAt   time.Time
}
```

---

## Status Vocabulary

### Health States

- `up`: Monitor is healthy and functioning
- `down`: Monitor is failing or unreachable
- `degraded`: Monitor is partially functional (computed by core)
- `unknown`: Health status is not yet determined

### Lifecycle States

- `active`: Monitor is active and being checked
- `disabled`: Monitor is temporarily disabled (not checked)
- `deleted`: Monitor is soft-deleted (can be revived)

---

## Error Handling

### Agent Error Handling

- **Network failures**: Retry with exponential backoff (max 3 attempts)
- **Authentication errors**: Log and stop reporting (requires re-registration)
- **Invalid responses**: Log warning and continue operation
- **Collection errors**: Log error but continue with other monitors

### Core Error Handling

- **Invalid requests**: Return `400 Bad Request` with error message
- **Authentication failures**: Return `401 Unauthorized`
- **Not found**: Return `404 Not Found`
- **Server errors**: Return `500 Internal Server Error` with error details
- **All errors**: Include request ID in response for tracing

---

## Versioning

- All public APIs are versioned under `/v1`
- Health check endpoint (`/health`) is unversioned
- Future versions will use `/v2`, `/v3`, etc.
- Breaking changes require new version

---

## Security

- **Authentication**: Token-based, permanent tokens (non-expiring)
- **Transport**: HTTPS recommended in production
- **Token Storage**: Agent stores token in `state.yaml` (should be protected)
- **Token Validation**: Core validates token matches agent ID on every request

---

## Best Practices

### Agent

1. **Idempotent Operations**: All registration operations are idempotent
2. **Graceful Degradation**: Continue operation even if some monitors fail
3. **Resource Efficiency**: Batch reports when possible (future enhancement)
4. **Error Logging**: Log all errors with context for debugging

### Core

1. **Soft Deletes**: Always use soft deletes for monitors (can be revived)
2. **Request Tracing**: Include request IDs in all logs
3. **Idempotent APIs**: All registration endpoints are idempotent
4. **State Reconciliation**: Core reconciles agent-declared state

---

## Future Enhancements

- Batch reporting (multiple reports in single request)
- WebSocket support for real-time updates
- Token rotation and revocation
- Compression for large payloads
- Rate limiting per agent
