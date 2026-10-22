# Getting started

Run Orion end to end in about ten minutes: start Core on a server you own, connect a server to watch,
add a monitor, and receive an alert. You need no knowledge of this codebase.

Prefer to have an AI assistant do it? See [Set up Orion with an AI assistant](ai-setup.md).

## What you are setting up

| Piece | What it is | Where it runs |
|---|---|---|
| **Core** | The Orion server: API, Console (the web UI), monitor worker, status pages and the database (SQLite). | One machine you own, in Docker. |
| **Node** | A small program that watches a server (CPU, disk, services, containers, logs) and reports to Core. | Each Linux or macOS machine you want to watch. Optional: Core can also check public URLs by itself. |

You need:

- A Linux or macOS machine with **Docker** and the **Docker Compose plugin** for Core
  (`docker compose version` must work), plus `curl` and `openssl`.
- For each machine you want to watch: Linux or macOS, `curl`, and root or `sudo` access (or use the
  per-user install, see [Node install and upgrade](deployment/node-install-upgrade.md)).
- The machines you watch must be able to reach Core's address over HTTP or HTTPS.

## 1. Start Core

Run this **on the server that will host Core** (a VPS, a home server or any Linux machine you control). One
command creates a folder, writes the settings with generated secrets, starts Core and waits until it is
healthy:

```sh
curl -fsSL https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/scripts/setup-core.sh | bash
```

The script works out the server's address and uses it as Core's public address. Add
`--public-url https://orion.example.com` if you reach Core through a domain, and `--dir /opt/orion` to
choose the folder. It ends by printing the Console address and your login, for example:

```text
Orion Core is running.

  Console   http://203.0.113.10:8999
  Login     admin  /  <generated password>
```

Open that address from your own computer and sign in. If it does not load, allow TCP port `8999` through the
server's firewall or cloud security group. You land on the first-run screen ("Nothing is monitored yet").

The password is stored in `orion/.env`. To change anything, edit that file and run
`docker compose up -d` in the `orion` folder.

**Prefer to do it by hand?**

```sh
mkdir orion && cd orion
curl -fsSLO https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/docker-compose.yml
curl -fsSL  https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/.env.example -o .env
```

Edit `.env` and set `ORION_ADMIN_PASSWORD` (for example `openssl rand -base64 24 | tr -d '/+='`),
`ORION_JWT_SECRET` (`openssl rand -hex 32`) and `ORION_PUBLIC_URL` (`http://<server-ip>:8999`, or your
domain). Optionally pin `ORION_CORE_IMAGE` to a release such as `ghcr.io/sunday-studio/orion-core:0.2.1`.
Then:

```sh
docker compose up -d
docker compose ps        # both services should say "healthy" within a minute
```

Every setting, including email alerts, status page mail and tuning, is documented inside `.env.example`
and in the [environment reference](deployment/core-environment.md).

**Using a domain and HTTPS.** Put a reverse proxy in front of Core (a Caddy example is in
[`deploy/examples/reverse-proxy`](../deploy/examples/reverse-proxy)), set `ORION_PUBLIC_URL` to the
public address, and set `ORION_TRUSTED_PROXIES` to the proxy's address. See
[Core security defaults](deployment/core-security-defaults.md).

## 2. Connect a server

In the Console open **Servers**, choose **Add server**, pick Linux or macOS and copy the command it shows.
Run it on the machine you want to watch. It looks like this:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh | \
  ORION_ENROLLMENT_TOKEN=<token> bash -s -- --core-url https://orion.example.com
```

The installer prints what it will do, downloads the Node, verifies its checksum, creates a service
and enrolls with Core. When it finishes, the Console shows the server as connected within a minute.

Check it on the server:

```sh
orion-node status     # the service is running and Core accepted its token
orion-node doctor     # a checklist: config, service, connection, clock, disk
```

If something fails, `orion-node doctor` names the problem and the command that fixes it. More commands:
[Server CLI cheat sheet](deployment/server-cli-cheatsheet.md).

You can skip this step to start: Core also monitors public websites and TCP ports on its own (step 3).

## 3. Add a monitor

In the Console choose **Monitors**, then **New monitor**:

- **HTTP:** a URL that should answer with a healthy status code. Start with your own site.
- **TCP:** a host and port that should accept connections.
- **Heartbeat:** a URL your job calls on a schedule; Orion alerts when it stops.
- **Node monitor:** a service, container or disk on a connected server.

Choose **Test now** before saving. The monitor appears on the Overview within a minute.

## 4. Get an alert

In **Alerts**, open **Channels**, add a **Webhook** (Slack, Discord or any URL) or an **Email** channel
(email needs the SMTP settings in `.env`), and use **Send test** to confirm it works.

To see a real incident, point an HTTP monitor at an address that fails (for example
`https://does-not-exist.invalid`), wait for it to turn red, and watch the incident open in **Incidents** and the
notification arrive. Delete the monitor when you are done.

## 5. Optional next steps

- **Status page:** **Status pages** → **Create** publishes a public page at `/status/<name>`.
- **Send logs from your own apps:** [Product logs](product-logs.md).
- **Back up the database:** [SQLite backup and restore](sqlite-backup-restore.md).
- **Upgrade:** pull the new image and run `docker compose up -d`; read the
  [upgrade notes](deployment/upgrade-notes-0.2.0.md) first when moving from a version before 0.2.0.
- **Remove a server:** `orion-node` uninstall steps are in
  [Node install and upgrade](deployment/node-install-upgrade.md).

## If something goes wrong

| Symptom | Try |
|---|---|
| `docker compose up` says a variable is not set | Set `ORION_ADMIN_PASSWORD` and `ORION_JWT_SECRET` in `.env`. |
| The Console does not load | `docker compose ps` and `docker compose logs orion-core` in the `orion` folder. |
| A server does not appear | Run `orion-node doctor` on it. The usual causes are a Core address the server cannot reach, a wrong token, or a firewall on port 8999. |
| A monitor of a private address fails with a blocked-target error | Core blocks private addresses by default; see `ORION_CORE_MONITOR_ALLOW_PRIVATE_TARGETS` in `.env.example`. |
| You forgot the password | It is in `orion/.env`. |

More help: [How Orion works](how-it-works.md) and the [deployment docs](deployment/README.md).

## Developing Orion itself

Contributors run the stack from source with `make docker-dev-up` and load demo data with
`make docker-dev-seed`; see [Local development](development/local-development.md).
