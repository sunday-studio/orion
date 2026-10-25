# Orion

[![CI](https://github.com/sunday-studio/orion/actions/workflows/ci.yml/badge.svg)](https://github.com/sunday-studio/orion/actions/workflows/ci.yml)

<p align="center">
  <img src="assets/orion-banner.png" alt="Orion: run monitoring and incident response on a machine you own" width="880">
</p>

**Self-hosted monitoring, incident response and logs for personal projects and small teams.**

**[Documentation](https://orion.thesunday.studio/docs/)** · [Quickstart](https://orion.thesunday.studio/docs/quickstart/) · [Set up with an AI assistant](https://orion.thesunday.studio/docs/ai-setup/) · [orion.thesunday.studio](https://orion.thesunday.studio)

Run Orion on a machine you own. When something breaks you hear it first, and every byte stays on your
server. There is no hosted version and no account to create.

<table>
  <tr>
    <td width="50%">
      <picture>
        <source media="(prefers-color-scheme: dark)" srcset="assets/console-overview-dark.png">
        <img src="assets/console-overview-light.png" alt="The Orion Console overview: an open incident, server health and recent incidents">
      </picture>
      <sub><b>Overview</b>: what needs you now, and how your servers are doing</sub>
    </td>
    <td width="50%">
      <picture>
        <source media="(prefers-color-scheme: dark)" srcset="assets/console-incident-detail-dark.png">
        <img src="assets/console-incident-detail-light.png" alt="An incident with its timeline, notifications and affected components">
      </picture>
      <sub><b>Incidents</b> with a timeline and who was told</sub>
    </td>
  </tr>
</table>

## What you get

- **Monitors** for HTTP endpoints, websites, TCP ports, Docker containers, systemd services, PM2
  processes, commands and resource thresholds.
- **Incidents** opened automatically when a monitor fails, with a timeline of what failed, who was told
  and who acknowledged.
- **Alerts** routed by rule to webhook or email, with every delivery recorded and failed ones retried.
- **Logs** from your own apps over JSON or OpenTelemetry, searchable next to the incident.
- **Status pages** you can publish for the people who depend on you.
- **One small footprint:** one API process, a monitor worker, SQLite, and the bundled Console. Back it up
  as ordinary files.

## Why Orion?

Most uptime tools run checks from outside your network. That is useful, but it cannot see local service
state, Docker containers, systemd units, PM2 processes, command checks, or the machine's own resource
pressure.

Orion keeps that local context on the monitored host. Each Server runs next to the services it watches
and reports to a Core you control, so you can run it from Docker Compose without adopting Prometheus,
Postgres, Kubernetes or a hosted observability stack first.

## Get started

About ten minutes, on the server that will host Core (any Linux machine with Docker). The full steps, with
every command, are in the **[Quickstart](https://orion.thesunday.studio/docs/quickstart/)** on the documentation site.

```sh
curl -fsSL https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/scripts/setup-core.sh | bash
```

That starts **Core** (the API, the Console and the worker) with Docker Compose and prints the address and
your login. Then:

1. **Connect a server** with the one-line installer the Console shows under Servers, Add server, on any Linux
   or macOS host you want to watch.
2. **Add a monitor and an alert channel**, then send a test message.

The settings are in [`deploy/.env.example`](deploy/.env.example); the compose file is
[`deploy/docker-compose.yml`](deploy/docker-compose.yml). Building Core from source on a new server:
[Set up Core with Docker](docs/deployment/docker-new-server.md).

Want to see it work before installing anything? The demo starts Core, monitors a tiny Python service,
breaks it and checks recovery:

```sh
examples/python-sleep-compose/smoke.sh
```

## Let your AI set it up

Hand the whole setup to an AI assistant. It asks you a few questions, then installs and verifies Core, connects
your servers and sends a test alert, asking before it changes anything.

[![Set up Orion with Claude](https://img.shields.io/badge/Set%20up%20Orion%20with-Claude-D97757?style=for-the-badge&logo=anthropic&logoColor=white)](https://claude.ai/new?q=Set%20up%20Orion%20%28self-hosted%20monitoring%29%20for%20me%20end%20to%20end.%20Read%20https%3A%2F%2Fraw.githubusercontent.com%2Fsunday-studio%2Forion%2Fmain%2Fdocs%2Fai-setup.md%20and%20follow%20it%20step%20by%20step.%20Ask%20me%20the%20questions%20it%20lists%20first%2C%20and%20ask%20before%20you%20change%20anything%20on%20a%20machine.)

Using Claude Code or another assistant that can run commands? Paste this:

```text
Set up Orion (self-hosted monitoring) for me end to end. Read
https://raw.githubusercontent.com/sunday-studio/orion/main/docs/ai-setup.md and follow it step by step.
Ask me the questions it lists first, and ask before you change anything on a machine.
```

The guide is on the [documentation site](https://orion.thesunday.studio/docs/ai-setup/), and the instructions the assistant follows are in [docs/ai-setup.md](docs/ai-setup.md): what to ask, the exact
commands, how to check every step, and when to stop.

## Documentation

**The documentation site is at [orion.thesunday.studio/docs](https://orion.thesunday.studio/docs/).** Start there: the quickstart, installing Core, adding a
server, sending logs, backups, the CLI and the API.

The in-repo reference, including deployment details and design notes, has its own map in
[docs/README.md](docs/README.md). The most useful pages:

| | |
|---|---|
| **Start** | [Quickstart](https://orion.thesunday.studio/docs/quickstart/) · [Set up with AI](https://orion.thesunday.studio/docs/ai-setup/) · [Getting started](docs/getting-started.md) · [How Orion works](docs/how-it-works.md) |
| **Run it** | [Deployment guide](docs/deployment/README.md) · [Core Docker deployment](docs/deployment/core-docker.md) · [Core security defaults](docs/deployment/core-security-defaults.md) · [Install and manage a Node](docs/deployment/node-install-upgrade.md) · [Server CLI cheat sheet](docs/deployment/server-cli-cheatsheet.md) |
| **Your data** | [Send logs from your apps](docs/product-logs.md) · [SQLite backup and restore](docs/sqlite-backup-restore.md) |
| **Build** | [Local development](docs/development/local-development.md) · [Architecture](docs/architecture/system-overview.md) · [System design](docs/system-design.md) |

## Project layout

```txt
orion/
├── apps/
│   ├── node/    # Go Server daemon and orion-node compatibility CLI
│   ├── core/     # Go API server, SQLite, OpenAPI, embedded Console
│   ├── console/  # React/Vite UI source
│   └── website/  # Astro site and documentation
├── deploy/       # Docker Compose, systemd, launchd, install scripts
├── docs/         # architecture, deployment, development, milestones
├── packages/     # shared/generated package space
└── Makefile
```
