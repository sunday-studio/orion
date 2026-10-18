---
title: Quickstart
description: "Run Core, connect a server and get your first alert."
group: Start
order: 2
---
You need a machine with Docker and Compose, and a Linux or macOS server to watch (it can be the same machine).

## 1. Start Core

Download the example Compose file, then set an admin password and a JWT secret in it (see [Install Core](/docs/install-core/) for every option).

```sh
curl -fsSL -o orion-compose.yaml \
  https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/examples/core-console-compose.yaml
```

Generate a secret and start it:

```sh
openssl rand -hex 32
docker compose -f orion-compose.yaml up -d
```

Open `http://localhost:8999` and sign in with the admin username and password you set.

## 2. Connect a server

Run the installer on the machine you want to watch. Point it at your Core.

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- \
  --core-url http://orion-core.local:8999
```

The server appears in the Console within a minute. Check it from the machine with `orion-agent status`. More in [Install a server](/docs/install-server/).

## 3. Add a monitor

In the Console, open **Monitors**, choose **New monitor**, pick HTTP, enter a URL and save. The first check runs right away.

## 4. Add an alert channel

Open **Alerts**, then **Channels**, and add a webhook or an email channel. Use **Send test** to confirm it arrives, then route your monitors to it.

## 5. Optional: send app logs

Create an app under **Settings**, copy its ingest key and send your first log. See [Send logs](/docs/send-logs/).
