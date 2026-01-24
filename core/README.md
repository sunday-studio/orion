# Orion Core Server

The Orion Core Server is the central hub that receives, authenticates, and stores telemetry reports from multiple Orion Agents running on external machines.

## Features

- **Agent Registration**: Secure registration of agents with permanent authentication tokens
- **Report Storage**: Reliable storage of incoming telemetry reports
- **Authentication**: Token-based authentication for secure communication
- **SQLite Database**: Lightweight, file-based database for simplicity
- **RESTful API**: JSON-based API endpoints
- **Structured Logging**: Comprehensive logging for observability

## API Endpoints

### 1. `POST /register`
Registers a new agent or returns existing agent information.

**Request Body:**
```json
{
  "uuid": "unique-agent-identifier",
  "name": "hostname-or-friendly-label",
  "os": "linux|mac|windows",
  "arch": "amd64|arm64|etc"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Agent registered successfully",
  "data": {
    "agent_id": 1,
    "token": "permanent-authentication-token"
  }
}
```

### 2. `POST /report/:agent_id`
Receives periodic status data from a registered agent.

**Headers:**
- `Authorization: Bearer <token>`

**Request Body:**
- Raw JSON payload (system metrics, service status, configs, etc.)

**Response:**
```json
{
  "success": true,
  "message": "Report received successfully",
  "data": {
    "message": "Report received successfully",
    "timestamp": "2024-01-01T12:00:00Z",
    "report_id": 0
  }
}
```

### 3. `GET /health`
Health check endpoint.

**Response:**
```json
{
  "status": "healthy",
  "service": "orion-core"
}
```

## Building and Running

### Prerequisites
- Go 1.25.3 or later

### Build
```bash
cd core
go mod tidy
go build -o orion-core .
```

### Run
```bash
./orion-core
```

The server listens on port **8999** by default.

### Docker

From the **repository root**:

```bash
# Build image
make docker-build
# or: docker build -f core/Dockerfile -t orion-core:latest .

# Run (no frontend auth)
docker run -p 8999:8999 -v orion-data:/data orion-core:latest

# Run with frontend login (set all three)
docker run -p 8999:8999 -v orion-data:/data \
  -e ORION_ADMIN_USERNAME=admin \
  -e ORION_ADMIN_PASSWORD=your-secret \
  -e ORION_JWT_SECRET=your-jwt-secret \
  orion-core:latest
```

Or with **docker compose** (`make docker-up` or `docker compose up -d`). Set `ORION_ADMIN_USERNAME`, `ORION_ADMIN_PASSWORD`, and `ORION_JWT_SECRET` in the environment or a `.env` file to enable frontend auth.

## Database

The server uses SQLite for data storage. The database file is created at `data/orion.db` and includes:

- **Agents Table**: Stores agent metadata and authentication tokens
- **Reports Table**: Stores all telemetry reports from agents

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `ORION_DATA_DIR` | `data` | Directory for SQLite (`orion.db`). Use `/data` in Docker. |
| `ORION_PORT` | `8999` | HTTP listen port. |
| `ORION_ADMIN_USERNAME` | — | If set with `ORION_ADMIN_PASSWORD`, enables frontend login. |
| `ORION_ADMIN_PASSWORD` | — | Admin password for the web UI. |
| `ORION_JWT_SECRET` | — | Required when frontend auth is on; used to sign JWTs. |

When both `ORION_ADMIN_USERNAME` and `ORION_ADMIN_PASSWORD` are set, the frontend requires login at `/login`. `ORION_JWT_SECRET` must also be set in that case.

## Security

- All agents receive permanent, non-expiring authentication tokens
- Tokens are validated on every report submission
- Invalid tokens result in 401 Unauthorized responses
- Agent IDs in URLs must match the token's associated agent

## Error Handling

- **400 Bad Request**: Malformed request payload
- **401 Unauthorized**: Missing or invalid token
- **404 Not Found**: Agent ID does not exist or token mismatch
- **500 Internal Server Error**: Database or server issues

## Logging

The server uses structured JSON logging with the following levels:
- **Info**: General operational messages
- **Error**: Error conditions
- **Debug**: Detailed debugging information
- **Warn**: Warning conditions

## Future Enhancements

- Web dashboard for agent monitoring
- Token revocation and rotation
- Message queue support for large-scale deployments
- API pagination and filtering
- Compressed or encrypted payloads
- Alerting system for stale agents
