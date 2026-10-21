# Set up Orion with an AI assistant

This page is written for an AI agent (Claude Code, Claude in a chat, or any assistant that can run shell
commands) that has been asked to **set up Orion end to end** for a person. If you are that person, give your
assistant this link, or use the button in the [README](../README.md#let-your-ai-set-it-up). If you are
the assistant, follow this page from the top.

Orion is self-hosted monitoring: **Core** (API, Console, worker, SQLite) runs in Docker on one machine, and
an optional **Node** program runs on each Linux or macOS server to watch. Human-oriented version of these
steps: [Getting started](getting-started.md).

## Ground rules for the assistant

1. **Ask before changing any machine.** Installing Docker, starting containers, writing to `/opt` or `/etc`,
   running anything with `sudo`, opening firewall ports and installing the Node on another server all need the
   person's clear yes. Explain what you will run first.
2. **Never print secrets back into chat unnecessarily.** The admin password lives in `.env`. Show it once, at
   the end, and tell the person where it is stored. Never commit `.env` or paste it into issues.
3. **Do not invent values.** If you do not know the public address, the SSH host or whether the person wants
   email alerts, ask. Do not guess a domain.
4. **Verify each step before moving on** with the check given for it. Stop and report on the first failure;
   do not retry blindly or work around a failed checksum, a refused connection or a denied permission.
5. **Use only the commands below** and the scripts they download from `github.com/sunday-studio/orion`.
   Read a script before running it if you can.
6. If a step fails, read [Troubleshooting](#troubleshooting) before improvising.

## 0. Ask these questions first

Collect the answers before running anything:

- **Where will Core run?** This machine, or another one reached over SSH? It needs Linux or macOS, Docker with
  the Compose plugin, `curl` and `openssl`.
- **What address will people and servers use to reach Core?** For a local trial, `http://localhost:8999` is
  fine. For real use, a domain with HTTPS behind a reverse proxy (see
  [Core security defaults](deployment/core-security-defaults.md)).
- **Which servers should be watched?** A list of hosts you may reach, or "none for now" (Core can still check
  public URLs).
- **How should alerts arrive?** A webhook URL (Slack, Discord, any HTTPS endpoint), email (needs SMTP
  details), or "decide later".
- **What should be monitored first?** For example their website URL.

## 1. Check the machine

```sh
uname -sm                      # Linux or Darwin; x86_64, aarch64 or arm64
docker --version && docker compose version
docker info >/dev/null && echo "docker daemon ok"
curl --version | head -n1 && openssl version
```

If Docker is missing, tell the person and point them to <https://docs.docker.com/engine/install/>. Do not
install it without their yes. If `docker info` fails with a permission error, the user needs the docker
group or `sudo`.

## 2. Start Core

```sh
curl -fsSL https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/scripts/setup-core.sh | \
  bash -s -- --dir ./orion --public-url <ADDRESS-FROM-STEP-0>
```

Omit `--public-url` for a local trial. The script checks Docker, downloads `docker-compose.yml` and
`.env.example`, writes `.env` with freshly generated secrets, pulls the image, starts Core and its worker, waits
for health, and prints the Console address and login.

**Verify:**

```sh
cd orion && docker compose ps                      # orion-core and orion-core-worker both "healthy"
curl -fsS http://127.0.0.1:8999/health             # prints a healthy status
```

The Console is at the printed address; the login is `admin` and the password in `orion/.env`. A fresh
instance shows "Nothing is monitored yet".

## 3. Connect each server to watch (optional)

On **each** machine to watch (Linux or macOS), with the person's approval and the Core address from step 0.
The shared enrollment token is `ORION_ENROLLMENT_TOKEN` in Core's `.env`:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh | \
  ORION_ENROLLMENT_TOKEN=<TOKEN> bash -s -- --core-url <CORE-ADDRESS>
```

Preview without changing anything by adding `--dry-run`. Without root, add `--user` for a per-user install.

**Verify on that server:**

```sh
orion-node status        # service running, token accepted by Core
orion-node doctor        # all checks ok; follow any "fix:" line it prints
```

The server appears under Servers in the Console within about a minute. Anything `doctor` flags includes the
command that fixes it. Full reference: [Node install and upgrade](deployment/node-install-upgrade.md).

## 4. Add the first monitor

Do this in the Console (Monitors, then New monitor) or tell the person exactly where to click. The assistant
cannot do it unless it has been given an API token; see [Product logs](product-logs.md) and the Console
Settings, then API tokens page if the person wants to automate this.

- HTTP monitor on the URL from step 0, then **Test now**, then save.
- Confirm it shows as up on the Overview.

## 5. Set up an alert channel and test it

Alerts, then Channels, then add a **Webhook** or **Email** channel and press **Send test**. Confirm the person
received the test message. Email needs the SMTP values in `.env` (see `.env.example`); after editing `.env`
run `docker compose up -d` in the `orion` folder.

## 6. Finish

Tell the person, in plain words:

- the Console address and login, and that the password is in `orion/.env`;
- which servers are connected and which monitors exist;
- how to back up (the `orion-data` Docker volume; see [SQLite backup and restore](sqlite-backup-restore.md));
- how to upgrade (change `ORION_CORE_IMAGE`, run `docker compose pull && docker compose up -d`; read the
  upgrade notes first).

## Troubleshooting

| Symptom | Cause and action |
|---|---|
| `setup-core.sh` fails on `docker info` | Docker daemon not running or no permission. Ask the person to start Docker or fix group membership. |
| Compose says `ORION_ADMIN_PASSWORD` is not set | `.env` missing or empty. Re-run the setup script or set the value from `.env.example`. |
| Core is `unhealthy` | `cd orion && docker compose logs orion-core`. Common causes: port 8999 taken (set `ORION_HTTP_PORT`), a bad value in `.env`, a read-only or full disk. |
| Pull fails or the image tag is missing | Check `ORION_CORE_IMAGE` in `.env` names a published version. |
| Installer fails verification (exit code 2) | A checksum or signature mismatch. **Stop.** Nothing was installed. Do not use `--skip-verify`; report it. |
| Node cannot reach Core | Run `orion-node doctor` on it. Check the Core address from that machine with `curl -fsS <CORE-ADDRESS>/health`, the firewall for the Core port, and the token. |
| A monitor on a private address is blocked | Core blocks private targets by default. Ask before enabling `ORION_CORE_MONITOR_ALLOW_PRIVATE_TARGETS`. |

## Reference for the assistant

- Core container: `orion-core` (API, Console, public status pages) and `orion-core-worker`; data in the
  `orion-data` volume; port `8999` by default.
- Health: `GET /health`. API docs and the contract between Node and Core:
  [docs/node-core-contract.md](node-core-contract.md).
- Settings: every environment variable is documented in `deploy/.env.example`.
- Source: <https://github.com/sunday-studio/orion>. Report problems at the repository's issue tracker.
