---
title: Install a server
description: "Install the Orion agent on Linux or macOS and manage it with the CLI."
group: Guides
order: 4
---
The agent is a small daemon that reports a machine's health, services and Docker containers to Core. It supports Linux (systemd) and macOS (launchd).

## Install

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- \
  --core-url http://orion-core.local:8999
```

The installer downloads the agent, verifies it, writes a config, registers the machine with Core and starts the service. If your Core requires an enrollment secret, pass the token the Console gives you.

## Paths

| | Linux | macOS |
|---|---|---|
| Config | `/etc/orion/config.yaml` | `/usr/local/etc/orion/config.yaml` |
| State | `/var/lib/orion/state.db` | `/usr/local/var/lib/orion/state.db` |
| Log | `/var/log/orion/agent.log` | `/usr/local/var/log/orion/agent.log` |
| Service | `/etc/systemd/system/orion-agent.service` | launchd |

## Verify

```sh
orion-agent status
orion-agent doctor
```

`doctor` checks the config, the connection to Core and the service, and tells you what to fix.

## Docker monitors on Linux

To watch containers, the `orion` user needs permission to read `/var/run/docker.sock`. Add it to the `docker` group and restart the agent.

## Update, roll back, uninstall

```sh
orion-agent update
orion-agent update -version 0.1.1
```

## Recover a token

If the agent can no longer authenticate, apply a replacement token and restart.

```sh
orion-agent token apply --token-file /secure/path/replacement-token
orion-agent restart
```

## Cloned machines

The agent keeps a `machine-identity.json` beside its state database. If you clone a VM or image, the hostname or machine id no longer matches, and the copy registers as a new, separate server rather than impersonating the original.
