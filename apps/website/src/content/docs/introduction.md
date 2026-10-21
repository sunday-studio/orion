---
title: Introduction
description: "What Orion is, what it watches and how the pieces fit together."
group: Start
order: 1
---
Orion is self-hosted monitoring for the servers, sites, jobs and apps you run. You run one **Core** on your own infrastructure. Servers and apps report to it, and you work from a single Console.

## What you get

- **Monitors** for HTTP and TCP targets, cron heartbeats and local server checks.
- **Incidents** opened automatically when a monitor fails, with a timeline of what happened and who was told.
- **Alerts** delivered to webhook and email channels, with every delivery recorded.
- **Status pages** you can publish for your users.
- **Product logs** from your apps, over a plain JSON endpoint or OpenTelemetry.
- **The Node**, a small program for Linux and macOS servers that watches the machine, its services and Docker containers from the inside.

## How it fits together

| Part | Role |
|---|---|
| **Core** | The API, the Console and the status page. Derives health, opens incidents and stores everything in SQLite. |
| **Worker** | Runs monitor checks and delivers alerts, in its own process so the API stays responsive. |
| **Node** | A small program you install on each server you want to watch. It reports to Core. |
| **Apps** | Anything that sends logs to Core with an ingest key. |

## What it is not

Orion is not a hosted service, a metrics or tracing backend, or a full observability suite. It covers logs, health, incidents and alerts. It runs on Linux and macOS, and stores data in one SQLite database.

## Next

Follow the [Quickstart](/docs/quickstart/) to have Core running and a first server connected in about ten minutes.
