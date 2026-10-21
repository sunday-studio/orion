# Set up Core on a new server with Docker

Run Core (with the Console and its monitor worker) as a Docker instance on one server, then connect
every machine you want to watch with the one-line agent installer. Allow about ten minutes.

You need a Linux server with Docker Engine and the Compose plugin (`docker compose version` works) and
`git`.

## 1. Set up Core

**Get the source and write your settings.** Core is built from this repository, so nothing is pulled from
a registry and nothing besides Docker is installed on the server.

```sh
git clone https://github.com/sunday-studio/orion.git
cd orion

cat > .env <<EOF
ORION_ADMIN_USERNAME=admin
ORION_ADMIN_PASSWORD=$(openssl rand -base64 24 | tr -d '/+=')
ORION_JWT_SECRET=$(openssl rand -hex 32)
ORION_ENROLLMENT_TOKEN=$(openssl rand -hex 16)
# The address your servers and you reach Core on; used in emails and install commands.
ORION_PUBLIC_URL=https://orion.example.com
EOF
chmod 600 .env
cat .env   # note the password: you sign in with it
```

| Setting | What it is |
|---|---|
| `ORION_ADMIN_USERNAME`, `ORION_ADMIN_PASSWORD` | The Console login. Core will not start without them. |
| `ORION_JWT_SECRET` | Signs sessions. 32 characters or more. |
| `ORION_ENROLLMENT_TOKEN` | Optional shared secret. When set, a server can only register with this value or a one-time token from the Console. Recommended. |
| `ORION_PUBLIC_URL` | The address people use to reach Orion. |
| `ORION_HTTP_PORT` | The port on the server. Defaults to `8999`. |

Keep `.env` private and out of version control.

**Build and start.** One shell function saves retyping the two compose files; use it for every command
in this guide (paste it into your shell, or `~/.bashrc` to keep it):

```sh
orion_compose() { docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.build.yml --env-file .env "$@"; }
orion_compose up -d --build
```

The first run builds the Core image (the Console is inside it), which takes a few minutes. Later runs
reuse the cache. This starts two services, `orion-core` and `orion-core-worker`.

**Check it.**

```sh
orion_compose ps
```

Both should say `healthy` within a minute. Open `http://<server address>:8999` and sign in with `admin`
and the password from `.env`.

### Put it behind TLS

Port `8999` is plain HTTP. Before you expose Core beyond a private network, put a reverse proxy with a
certificate in front of it and set `ORION_PUBLIC_URL` to the HTTPS address. A Caddy example is in
[`deploy/examples/reverse-proxy`](../../deploy/examples/reverse-proxy/). See also
[Core security defaults](core-security-defaults.md). Servers must be able to reach the address you give
them, so open the port or the proxy to them.

## 2. Connect your servers

On each machine you want to watch, run the installer. In the Console, **Servers, Add server** shows this
command with a one-time token already filled in; copy it from there. It looks like:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh \
  | sudo ORION_ENROLLMENT_TOKEN='<token>' bash -s -- --core-url https://orion.example.com
```

The installer downloads the agent, verifies it, installs it as a service (systemd on Linux, launchd on
macOS), registers the machine with Core and starts reporting. It asks no questions. The server appears
under **Servers** within a minute. Then run `orion-agent setup` on it to choose what to monitor; see
[Server install and upgrade](agent-install-upgrade.md) and the [CLI cheat sheet](server-cli-cheatsheet.md).

You can use `ORION_ENROLLMENT_TOKEN` from `.env` in place of a one-time token, until you replace the
registration secret in the Console's settings; after that, use one-time tokens.

### The server Core runs on

Install the agent there like any other server. If you would rather keep everything in Docker, start an
agent container beside Core instead. Add `COMPOSE_PROFILES=agent` to `.env` (it applies to every command,
so `ps`, `logs` and `down` include the agent too), then run `orion_compose up -d --build` again. What the
container can and cannot see is in [Running the Agent as a container](agent-container.md).

## Day to day

Run these from the repository root, with `orion_compose` defined as above.

| To | Run |
|---|---|
| See logs | `orion_compose logs -f orion-core` |
| Stop | `orion_compose down` (your data stays in the volume) |
| Update | `git pull`, then `orion_compose up -d --build` |
| Back up | see [SQLite backup and restore](../sqlite-backup-restore.md); the database is in the `orion-data` volume |

When an update has database migrations to apply, Core first copies the database to `backups/` inside the
`orion-data` volume (`/data/backups`), so you can roll back.

## If something is wrong

- **The Console will not load or sign in.** Check that Core is `healthy` (`orion_compose ps`), that you
  open it on the address and scheme your proxy serves (not `http` for an `https` site), and that
  `ORION_PUBLIC_URL` matches. Only a Console hosted separately from Core needs `ORION_CORS_ORIGINS`; see
  [Core Docker deployment](core-docker.md).
- **A server never appears.** On that server run `orion-agent doctor`; it names what is wrong. The usual
  causes are a Core address the server cannot reach and a firewall.
- **`address already in use`.** Something else uses port 8999. Set `ORION_HTTP_PORT` in `.env`.
- **The build fails or is killed.** The Console build needs a fair amount of memory. Build on a larger
  machine and move the image with `docker save` and `docker load`.
