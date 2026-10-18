---
title: Agent CLI
description: "Commands for managing the Orion agent on a server."
group: Reference
order: 8
---
| Command | What it does |
|---|---|
| `orion-agent status` | Show whether the service is running and connected to Core. |
| `orion-agent doctor` | Diagnose config, connectivity and the service, and suggest fixes. |
| `orion-agent restart` | Restart the service. |
| `orion-agent logs` | Show the agent's recent log entries. |
| `orion-agent update` | Update to the latest release. |
| `orion-agent token apply` | Apply a replacement token from a file. |
| `orion-agent reconfigure` | Create a new local registration. Only when you intend to. |

## Logs

```sh
orion-agent logs
orion-agent logs --lines 200
orion-agent logs --level error
orion-agent logs --since 1h
```

The command pretty-prints the agent's structured entries and falls back to systemd or launchd output when none exist.

## Update

```sh
orion-agent update
orion-agent update -version 0.1.2
```

## Token recovery

```sh
orion-agent token apply --token-file /secure/path/replacement-token
orion-agent restart
```

Use `reconfigure` only when you really need a new registration: it makes Core see the machine as a new server.
