# First Run Checklist

Use this checklist for a first self-hosted Orion run with one Core and one Server.

If you want a quick local proof before installing a Server service, run the
[`examples/python-sleep-compose`](../../examples/python-sleep-compose/) smoke first. It shows a real
healthy service, a forced failing service, Console verification, and reset commands.

## Core

- [ ] Download the Docker Compose sample:

```sh
curl -fsSL -o orion-compose.yaml \
  https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/docker-compose.yml
```

- [ ] Create `.env` next to `orion-compose.yaml` and set real values for:

```txt
ORION_REQUIRE_FRONTEND_AUTH=true
ORION_ADMIN_USERNAME
ORION_ADMIN_PASSWORD
ORION_JWT_SECRET
```

Use an admin password with at least 12 characters and a JWT secret with at least 32 characters.
Do not use the placeholder values shown in examples.

- [ ] Start Core:

```sh
docker compose -f orion-compose.yaml up -d
```

- [ ] Check health:

```sh
curl http://localhost:8999/health
```

- [ ] Open `http://localhost:8999` and sign in with the configured admin credentials.
- [ ] Confirm the `orion-data` Docker volume exists and is included in backups.

## Server

- [ ] Pick a Core URL the Server machine can reach, such as:

```txt
http://orion-core.local:8999
http://192.168.x.y:8999
http://100.x.y.z:8999
```

- [ ] Install from the Console: **Servers, Add server**, pick the platform and run the command it shows
  on the Server machine. It carries a one-time token, so nothing else needs to be set:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh \
  | sudo ORION_ENROLLMENT_TOKEN='orn_enr_...' bash -s -- --core-url http://orion-core.local:8999
```

Use the Core URL this Server host can reach. The expected output is in
[Install and manage a Node](node-install-upgrade.md).

- [ ] Or install with the sample config:

```sh
curl -fsSL -o orion-node-config.yaml \
  https://github.com/sunday-studio/orion/releases/latest/download/orion-node-config.yaml

curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh \
  | sudo ORION_ENROLLMENT_TOKEN='orn_enr_...' bash -s -- --config ./orion-node-config.yaml
```

- [ ] Confirm the Server service is running.
- [ ] Confirm local Server state exists:

Linux:

```sh
sudo test -f /var/lib/orion/state.db
```

macOS:

```sh
sudo test -f /usr/local/var/lib/orion/state.db
```

- [ ] Confirm local Server logs exist:

Linux:

```sh
sudo test -f /var/log/orion/node.log
orion-node logs --lines 20
```

macOS:

```sh
sudo test -f /usr/local/var/log/orion/node.log
orion-node logs --lines 20
```

- [ ] Confirm the Server appears in the Console Servers view.
- [ ] Confirm monitor rows appear after the first monitor interval.
- [ ] Restart the Server and confirm it does not create duplicate servers or monitors.

## Backup

- [ ] Run a SQLite backup after the first successful report. Core has two databases, `orion.db` and `logs.db`:

```sh
docker compose -f orion-compose.yaml exec orion-core sqlite3 /data/orion.db ".backup '/data/orion-backup.db'"
docker compose -f orion-compose.yaml exec orion-core sqlite3 /data/logs.db ".backup '/data/logs-backup.db'"
docker cp "$(docker compose -f orion-compose.yaml ps -q orion-core):/data/orion-backup.db" ./orion-backup.db
docker cp "$(docker compose -f orion-compose.yaml ps -q orion-core):/data/logs-backup.db" ./logs-backup.db
```

- [ ] Store the backups somewhere outside the Docker volume. Restore steps are in
      [SQLite backup and restore](../sqlite-backup-restore.md).
