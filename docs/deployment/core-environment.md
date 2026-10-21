# Core environment variables

Every `ORION_*` variable Core reads. Core is two programs sharing one `/data` volume: the **API** (`orion-core`: Console, API, public status pages) and the **Worker** (`orion-core-worker`: monitor checks and alert delivery). The "Used by" column says which one reads the variable, so you know which container needs it.

A new install needs only the three required variables below. Everything else has a safe default.

## Required

| Variable | Used by | What it does |
|---|---|---|
| `ORION_ADMIN_USERNAME` | API | Console sign-in name. |
| `ORION_ADMIN_PASSWORD` | API | Console password. At least 12 characters; placeholders such as `change-me` are refused. |
| `ORION_JWT_SECRET` | API | Signs Console sessions. At least 32 random characters. `openssl rand -base64 32` makes one. |

The three must be set together. With `ORION_REQUIRE_FRONTEND_AUTH=true` (the Compose files set it) Core refuses to start if any is missing or weak. With none of them and no opt-out Core refuses to start at all.

## Server and security

| Variable | Default | Used by | What it does |
|---|---|---|---|
| `ORION_DATA_DIR` | `data` (`/data` in Docker) | both | Where `orion.db`, `logs.db`, the report archive and `backups/` live. |
| `ORION_PORT` | `8999` | API | Listen port. |
| `ORION_LISTEN_HOST` | empty (all interfaces) | API | Bind address. Ignored, and forced to `127.0.0.1`, when Core runs without authentication. |
| `ORION_REQUIRE_FRONTEND_AUTH` | `false` (`true` in Compose) | both | Refuse to start unless sign-in is configured and strong. |
| `ORION_ALLOW_INSECURE_NO_AUTH` | `false` | API | Run with every admin route open. Loopback only. For local experiments. |
| `ORION_SESSION_HOURS` | `12` | API | How long a Console sign-in lasts. |
| `ORION_TRUSTED_PROXIES` | empty | API | Comma-separated proxy addresses whose `X-Forwarded-For` Core trusts. Set it behind a reverse proxy, otherwise rate limits see the proxy. |
| `ORION_CORS_ORIGINS` | `http://localhost:5173,http://127.0.0.1:5173` | API | Browser origins allowed to call the API. The bundled Console is same-origin and needs none. |
| `ORION_ENABLE_SWAGGER` | `false` | API | Serve the Swagger UI. |
| `ORION_METRICS_TOKEN` | empty | API | A Prometheus-style bearer token for `/metrics`. Empty means `/metrics` needs a normal admin sign-in, which scrapers cannot do, so set it if you scrape. |
| `ORION_LOGIN_RATE_LIMIT_ATTEMPTS` | `5` | API | Failed sign-ins allowed per window and client. `0` turns the limit off. |
| `ORION_LOGIN_RATE_LIMIT_WINDOW_SECONDS` | `60` | API | The window for the limit above. |
| `ORION_REGISTER_RATE_LIMIT_PER_MINUTE` | `30` | API | Node registrations per minute and client IP. `0` turns the limit off. |
| `ORION_ENROLLMENT_TOKEN` | empty | API | Shared registration secret for nodes. Empty means any caller that can reach `/v1/register` may register a node (rate limited). Set it, or only hand out the Console's one-time Add server commands, on any Core that is reachable from the internet. A token saved in Settings wins over this variable. |
| `ORION_PUBLIC_URL` | empty | both | The address nodes and email links use to reach Core, for example `https://orion.example.com`. Also settable in Settings, where it wins. Needed for Add server commands and email links on anything but `localhost`. |
| `ORION_DEADMAN_URL` | empty | API | A URL (healthchecks.io style) Core pings on a timer while it runs, so an outside service alerts you when the pings stop. |
| `ORION_DB_LOG_LEVEL` | `warn` | both | SQL logging: `silent`, `error`, `warn` (slow queries and errors) or `info`. Values are never logged. |

## Worker and monitors

| Variable | Default | Used by | What it does |
|---|---|---|---|
| `ORION_WORKER_ID` | `core-monitor-worker` | Worker | Name the Worker reports under. |
| `ORION_WORKER_HEARTBEAT_SECONDS` | `15` | Worker | How often it reports it is alive. |
| `ORION_WORKER_STALE_SECONDS` | `60` | both | After this long without a heartbeat the Worker counts as stale. |
| `ORION_WORKER_CONCURRENCY` | `16` | Worker | Checks run at once (0 to 256). |
| `ORION_CORE_MONITOR_ALLOW_PRIVATE_TARGETS` | `false` | both | Let Core-run monitors reach localhost and private networks. The API needs the same value as the Worker because Console "Test now" runs inside the API. Cloud metadata and link-local addresses stay blocked. |
| `ORION_PLAYWRIGHT_RUNNER` | empty | Worker | Path to a trusted Playwright runner executable. Empty makes browser monitors fail with `runtime_unavailable`; every other monitor type is unaffected. |
| `ORION_DATA_LIFECYCLE_SCHEDULER_SECONDS` | `3600` | API | How often retention and archiving jobs run. |
| `ORION_INTERNAL_LOG_RETENTION_DAYS` | `7` | API | How long Core keeps its own log lines. |
| `ORION_INTERNAL_LOG_MAX_STORAGE_MB` | `200` | API | Size cap for Core's own log lines. |
| `ORION_LOGS_QUEUE_MAX` | `50000` | both | Product log lines buffered before the oldest are dropped. |
| `ORION_LOGS_APP_RATE_PER_SECOND` | `1000` | API | Default ingest limit per app (each app can override it). |

## Alerts

| Variable | Default | Used by | What it does |
|---|---|---|---|
| `ORION_ALERT_ALLOW_PRIVATE_WEBHOOKS` | `false` | both | Let webhook alerts reach LAN or loopback receivers such as ntfy or Home Assistant. |
| `ORION_ALERT_COOLDOWN_SECONDS` | `300` | Worker | Minimum gap between repeats of the same alert for one incident. A rule can override it. |
| `ORION_ALERT_RECOVERY_NOTIFICATIONS` | `true` | Worker | Also notify when an incident resolves. |
| `ORION_ALERT_TLS_EXPIRY_DAYS` | `14` | Worker | Warn this many days before a certificate expires. |
| `ORION_ALERT_SMTP_HOST` | empty | both | SMTP server for email alert channels. Email alerts stay off until it is set. |
| `ORION_ALERT_SMTP_PORT` | `587` | both | SMTP port. |
| `ORION_ALERT_SMTP_USERNAME` | empty | both | SMTP login. |
| `ORION_ALERT_SMTP_PASSWORD` | empty | both | SMTP password. `ORION_ALERT_SMTP_PASSWORD_FILE` reads it from a file instead (Docker or Kubernetes secrets). |
| `ORION_ALERT_SMTP_FROM` | empty | both | Sender address. |
| `ORION_ALERT_SMTP_TLS` | `starttls` | both | `starttls`, `tls` or `none`. |
| `ORION_ALERT_EMAIL_SECRET` | empty | both | Signs the confirm and unsubscribe links in alert email. `ORION_ALERT_EMAIL_SECRET_FILE` reads it from a file. **Set it whenever `ORION_ALERT_SMTP_HOST` is set**, on both the API and the Worker, with the same value. The Worker has no `ORION_JWT_SECRET` in the Compose files, so without it the Worker cannot sign the links and every email delivery fails with "email alerts need ORION_PUBLIC_URL and ORION_ALERT_EMAIL_SECRET" (monitoring itself keeps running). `ORION_PUBLIC_URL` is needed too. |

## Public status pages

Public status page subscriber mail uses its own sender and is off by default. Pages, badges and feeds work without it.

| Variable | Default | Used by | What it does |
|---|---|---|---|
| `ORION_PUBLIC_STATUS_MAIL_ENABLED` | `false` | API | Send subscriber confirmation and incident mail. Needs the host, from address, origin and subscriber secret below. |
| `ORION_PUBLIC_STATUS_MAIL_HOST` | empty | API | SMTP server. |
| `ORION_PUBLIC_STATUS_MAIL_PORT` | `587` | API | SMTP port. |
| `ORION_PUBLIC_STATUS_MAIL_USERNAME` | empty | API | SMTP login. |
| `ORION_PUBLIC_STATUS_MAIL_PASSWORD` | empty | API | SMTP password. |
| `ORION_PUBLIC_STATUS_MAIL_FROM_EMAIL` | empty | API | Sender address. |
| `ORION_PUBLIC_STATUS_MAIL_FROM_NAME` | `Orion Status` | API | Sender name. |
| `ORION_PUBLIC_STATUS_MAIL_REPLY_TO` | empty | API | Reply-To address. |
| `ORION_PUBLIC_STATUS_URL_ORIGIN` | empty | API | Public origin of the status pages, for example `https://status.example.com`, used in email links. |
| `ORION_PUBLIC_STATUS_SUBSCRIBER_SECRET` | empty | API | Encrypts subscriber addresses. Without it Core stores only hashes, so operators cannot see, export or import full addresses. |
| `ORION_STATUS_PAGES_CNAME_TARGET` | the host of the status origin, then of `ORION_PUBLIC_URL` | API | The host custom status page domains must point a CNAME at. |

## Image-level

`GIN_MODE=release` is set in the image. Docker Compose also reads `ORION_CORE_IMAGE`, `ORION_HTTP_PORT` and the `ORION_CORE_*` and `ORION_WORKER_*` memory and CPU limits; those are Compose settings, not Core settings. See [Core Docker deployment](core-docker.md).
