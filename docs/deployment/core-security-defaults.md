# Core security defaults

## Where to run it

Core ships as one Docker image that runs as two services sharing a `/data` volume: the API with the
Console, and the monitor worker. Put it somewhere your Servers can always reach, such as a small VPS, a
Tailscale-reachable host, or a home server that is more reliable than the things it watches.

## Defaults worth knowing

- `ORION_REQUIRE_FRONTEND_AUTH=true` makes Core refuse to start unless login is configured with an admin
  password of at least 12 characters and a JWT secret of at least 32. Keep both out of source control.
- Behind a reverse proxy, set `ORION_TRUSTED_PROXIES` to the proxy address so rate limits see real client
  IPs. Without it Core ignores `X-Forwarded-For`.
- Core refuses to start with no admin credentials. For local experiments only, set
  `ORION_ALLOW_INSECURE_NO_AUTH=true`; every admin route is then open to anyone who can reach the port.
- Swagger UI is off unless `ORION_ENABLE_SWAGGER=true`.
- Request bodies are limited (1 MiB, or 5 MiB for NDJSON log batches), and URL paths with embedded tokens are not
  logged.
- Node registration is open to anyone who can reach Core unless you set `ORION_ENROLLMENT_TOKEN` (or a
  registration token in Settings). Console "Add server" commands carry a one-time token that works either
  way, and `/v1/register` is rate limited, but set a token on any Core reachable from the internet.
- Public status page subscriber email uses its own sender (`ORION_PUBLIC_STATUS_MAIL_*`). Email alert
  channels use `ORION_ALERT_SMTP_*` and need `ORION_ALERT_EMAIL_SECRET` shared by the API and the Worker;
  without it the Worker, which has no JWT secret, cannot sign alert email links and email deliveries fail.
- `/metrics` needs an admin sign-in unless `ORION_METRICS_TOKEN` is set, which is the token to give a scraper.

Every variable, its default and which container reads it: [Core environment variables](core-environment.md).

Plain `docker run` commands, backup, CORS and upgrades are in
[Core Docker deployment](core-docker.md).
