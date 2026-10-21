---
title: Concepts
description: "The words Orion uses: servers, monitors, incidents, channels, status pages and apps."
group: Guides
order: 5
---
## Server and node

A **server** is a machine you watch. The **node** is the program on it that reports to Core. In the Console you work with servers; in the terminal you work with the node.

## Monitors

A monitor is a recurring check. HTTP and TCP monitors run from Core. A heartbeat monitor expects your job to ping Orion on a schedule, and fails when it does not. Node monitors check local things such as services and containers.

Monitors can be paused or put in maintenance, so planned work does not open incidents.

## Incidents

When a monitor fails, Orion opens an **incident**. Related failures are grouped, a timeline records each step, and the incident resolves when the checks recover. You can also declare an incident yourself, and post updates to a status page.

## Alerts

An **alert rule** decides which events matter. A **route** sends them to a **channel**, which is a webhook or an email destination. Every attempt is stored as a **delivery**, so you can see what was sent, when, and whether it arrived.

## Status pages

A status page shows chosen components and the incidents that affect them. You can preview it, attach a domain and post public updates.

## Apps and logs

An **app** is anything that sends logs. Each app has write-only **ingest keys**, retention and redaction settings. See [Send logs](/docs/send-logs/).
