# Upgrade notes: backend hardening release

Behavior changes an operator should know before upgrading. Migrations are forward-only; Core writes a backup of an existing database to `$ORION_DATA_DIR/backups/` before applying them (see [sqlite-backup-restore.md](../sqlite-backup-restore.md)).

## Breaking or default-changing

| Change | What to do |
|---|---|
| **Core refuses to start without admin credentials** | Set `ORION_ADMIN_USERNAME`, `ORION_ADMIN_PASSWORD` and `ORION_JWT_SECRET`, or opt out explicitly with `ORION_ALLOW_INSECURE_NO_AUTH=true` (then Core binds to loopback only). Docker Compose already sets credentials. |
| **Alert webhooks to private/loopback addresses are blocked** (home-lab receivers such as ntfy or Home Assistant on a LAN) | Set `ORION_ALERT_ALLOW_PRIVATE_WEBHOOKS=true` (passed through by the Compose file). Cloud-metadata and link-local addresses stay blocked. Existing channels are re-checked at delivery time, so a blocked channel shows a failed delivery. |
| **Proxy environment variables are ignored for webhooks and Core monitors** | The SSRF policy must dial the real target. If you relied on `HTTP(S)_PROXY` for these, run a receiver reachable directly. |
| **The Worker is required for scheduled delivery** | Alerts are no longer sent inside the ingestion request; the Worker's dispatcher sends them (the API process takes over while no Worker is online and logs a warning). Run `orion-core-worker`. |
| **Maintenance mode always ends** | Maintenance enabled before the upgrade ends 24h after it. Agents already in local maintenance have no end recorded and stay paused until `orion-agent maintenance up`; re-issue `maintenance down --for <duration>` where you still need it, or those hosts page as stale once Core expires the window. |
| **Plaintext agent tokens are converted to hashes at startup** | Registered agents keep working. Re-registration of an existing machine now returns `409 agent_token_reissue_required`; reissue a token from the Console. |
| **`/v1/register` is rate limited** (30/min per client IP) and can require `ORION_ENROLLMENT_TOKEN` | Optional: set an enrollment secret and distribute it with `ORION_ENROLLMENT_TOKEN` to new installs. Behind a reverse proxy set `ORION_TRUSTED_PROXIES`, otherwise every client appears to be the proxy. |
| **Swagger UI is off** | Set `ORION_ENABLE_SWAGGER=true` to serve it. |
| **Stricter request limits** | Bodies over 1 MiB (5 MiB for log batches) return 413; list endpoints clamp `limit` to 200 and uptime `period` to 365d. |
| **Agent installer verifies checksums** | Releases published before `SHA256SUMS` existed need `--skip-verify`. Pin `--version` in production. |
| **Container runs as uid 10001** | The entrypoint takes ownership of an existing root-owned `/data` volume on first start. |

## New things to configure (all optional)

`ORION_METRICS_TOKEN`, `ORION_DEADMAN_URL`, `ORION_WORKER_CONCURRENCY`, `ORION_DB_LOG_LEVEL`, retention windows in the data lifecycle settings.

## Rollback

Redeploy the previous image and restore the newest pre-migration backup. The unique active-incident index and foreign keys are the constraints that can block running the old binary against the new schema; restoring the backup avoids that.
