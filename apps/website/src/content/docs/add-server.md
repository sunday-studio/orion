---
title: Add a server
description: "Install the Orion Node on a Linux or macOS machine so Core can watch it."
group: Guides
order: 4
---
A **server** is a machine you want Orion to watch. You add it in the Console, then install the **Node** on it: a small program that reports the machine's health, services and Docker containers to Core. It supports Linux (systemd) and macOS (launchd). Core needs to be running first, see [Install Core](/docs/install-core/).

## Add the server

1. In the Console, open **Servers** and choose **Add server**.
2. Copy the install command. It carries a one-time token that lets the Node register.
3. Run it on the machine you want to watch.

## Install the Node

The command from the Console looks like this:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh | bash -s -- \
  --core-url http://orion-core.local:8999
```

The installer downloads the Node, verifies it, writes a config, registers the machine with Core and starts the service. If your Core requires an enrollment secret, pass the token the Console gives you. The server appears under **Servers** within a minute.

## Paths

| | Linux | macOS |
|---|---|---|
| Config | `/etc/orion/config.yaml` | `/usr/local/etc/orion/config.yaml` |
| State | `/var/lib/orion/state.db` | `/usr/local/var/lib/orion/state.db` |
| Log | `/var/log/orion/node.log` | `/usr/local/var/log/orion/node.log` |
| Service | `/etc/systemd/system/orion-node.service` | launchd |

## Verify

```sh
orion-node status
orion-node doctor
```

`doctor` checks the config, the connection to Core, discovery, logs and the service, and prints the exact command that fixes anything wrong.

The installer asks no questions: when it finishes, CPU, memory, disk and network already report. `orion-node setup` is optional and lets you pick what else to monitor from what it found on the server.

## Watch Docker containers on Linux

To watch containers, the `orion` user needs permission to read `/var/run/docker.sock`. Add it to the `docker` group and restart the Node.

## Update, roll back, uninstall

```sh
orion-node update
orion-node update --version 0.1.1
```

## Recover a token

If the Node can no longer authenticate, apply a replacement token and restart.

```sh
orion-node token apply --token-file /secure/path/replacement-token
orion-node restart
```

## Cloned machines

The Node keeps a `machine-identity.json` beside its state database. If you clone a VM or image, the hostname or machine id no longer matches, and the copy registers as a new, separate server rather than impersonating the original.
