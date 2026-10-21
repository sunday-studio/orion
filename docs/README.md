# Orion documentation

Orion is self-hosted monitoring, incident response and logs for personal projects and small teams.
This is the map of the documentation in this repository. The public documentation site lives in
[apps/website](../apps/website/).

## Start here

- [Getting started](getting-started.md): Core, a Server and your first alert in about ten minutes.
- [How Orion works](how-it-works.md): the pieces, the diagram and what it can watch.

## Run it

- [Deployment guide](deployment/README.md) and [first run checklist](deployment/first-run-checklist.md)
- [Core Docker deployment](deployment/core-docker.md)
- [Core security defaults](deployment/core-security-defaults.md)
- [Server install and upgrade](deployment/node-install-upgrade.md)
- [Server CLI cheat sheet](deployment/server-cli-cheatsheet.md)
- [SQLite backup and restore](sqlite-backup-restore.md)
- [Send logs from your apps](product-logs.md)
- [Kubernetes position](deployment/kubernetes-position.md)

## Understand it

- [System design](system-design.md)
- [Architecture overview](architecture/system-overview.md) and the [architecture index](architecture/README.md)
- [Core features](architecture/core-features.md)
- [Data ingestion](architecture/data-ingestion.md)
- [Persistence and lifecycle](architecture/persistence-and-lifecycle.md)
- [Incident reconciliation](architecture/incident-reconciliation-flow.md)
- [Server monitors](architecture/node-monitors.md)

## Build and contribute

- [Local development](development/local-development.md)
- [Seed demo data](development/seed-demo-data.md)
- [Incident readiness PR checklist](development/incident-readiness-pr-checklist.md)
- [Release readiness gate](deployment/release-readiness.md)
- [Milestones](milestones/README.md)
