---
title: Install Core
description: "Run Core and its worker with Docker Compose, and configure authentication."
group: Guides
order: 3
---
Core ships as one image, `ghcr.io/sunday-studio/orion-core:<version>`, run as two services from the same volume: `orion-core` (API, Console and status page) and `orion-core-worker` (monitor checks and alert delivery). They must share one host and one data volume.

## Compose

Start from [`deploy/examples/core-console-compose.yaml`](https://github.com/sunday-studio/orion/blob/main/deploy/examples/core-console-compose.yaml) and edit the environment.

```sh
docker compose -f orion-compose.yaml up -d
docker compose -f orion-compose.yaml ps
```

Core listens on port `8999` by default.

## Required settings

Turn on authentication for the Console and set the admin account.

| Variable | Meaning |
|---|---|
| `ORION_REQUIRE_FRONTEND_AUTH` | Set to `true` to require sign-in for the Console. |
| `ORION_ADMIN_USERNAME` | Admin username. |
| `ORION_ADMIN_PASSWORD` | Admin password, at least 12 characters. |
| `ORION_JWT_SECRET` | Secret used to sign sessions, at least 32 characters. Generate with `openssl rand -hex 32`. |
| `ORION_HTTP_PORT` | HTTP port. Default `8999`. |

## Verify

```sh
curl http://localhost:8999/health
curl http://localhost:8999/v1/diagnostics/core-worker
```

The second endpoint (admin sign-in required) reports whether the worker is alive. If monitors are not running, check it first.

## Monitor targets

Core checks HTTP and TCP targets from its own network position. A policy controls which addresses it may reach, so a monitor cannot be pointed at internal addresses by accident. Review the policy in the repository's `docs/deployment/core-docker.md` before monitoring private hosts.

## Reverse proxy

Put Core behind a reverse proxy for TLS. Examples are in [`deploy/examples/reverse-proxy`](https://github.com/sunday-studio/orion/tree/main/deploy/examples/reverse-proxy).

## Upgrading

Pull the new image tag and recreate both services. Core applies embedded migrations on start, and writes a consistent copy of the database to `backups/` first. See [Backups](/docs/backups/) for rollback.
