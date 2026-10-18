# Getting started

About ten minutes, on a Linux or macOS machine with Docker.

**1. Start Core.**

```sh
curl -fsSL -o orion-compose.yml \
  https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/docker-compose.yml

ADMIN_PASSWORD="$(openssl rand -base64 24)"
JWT_SECRET="$(openssl rand -base64 32)"

cat > .env <<'EOF'
ORION_CORE_IMAGE=ghcr.io/sunday-studio/orion-core:<version>
ORION_HTTP_PORT=8999
ORION_REQUIRE_FRONTEND_AUTH=true
ORION_ADMIN_USERNAME=admin
EOF
printf 'ORION_ADMIN_PASSWORD=%s\nORION_JWT_SECRET=%s\n' "$ADMIN_PASSWORD" "$JWT_SECRET" >> .env

docker compose -f orion-compose.yml up -d
```

Open `http://localhost:8999` and sign in with `admin` and the password in `.env`.

**2. Connect a server.** Run this on any Linux or macOS host you want to watch, using a Core URL that host
can reach:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- \
  --core-url https://core.your-domain.tld
```

**3. Add a monitor and an alert channel** in the Console, then send a test message.

Want to see it work before installing anything? Run the demo:
[examples/python-sleep-compose](../examples/python-sleep-compose/) starts Core, monitors a tiny Python
service, breaks it, and checks recovery.

```sh
examples/python-sleep-compose/smoke.sh
```

## Next steps

- Understand the pieces: [How Orion works](how-it-works.md).
- Harden and tune Core: [Core security defaults](deployment/core-security-defaults.md) and
  [Core Docker deployment](deployment/core-docker.md).
- Day-to-day Server commands: [Server CLI cheat sheet](deployment/server-cli-cheatsheet.md).
- Send logs from your apps: [Product logs](product-logs.md).
- Back up the database: [SQLite backup and restore](sqlite-backup-restore.md).
