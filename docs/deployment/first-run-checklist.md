# First Run Checklist

Use this checklist for a first self-hosted Orion run with one Core and one Server.

## Core

- [ ] Download the Docker Compose sample:

```sh
curl -fsSL -o orion-compose.yaml \
  https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/examples/core-console-compose.yaml
```

- [ ] Edit `orion-compose.yaml` and set real values for:

```txt
ORION_ADMIN_USERNAME
ORION_ADMIN_PASSWORD
ORION_JWT_SECRET
```

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

- [ ] Install with a minimal config:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- \
  --core-url http://orion-core.local:8999
```

Use the Core URL this Server host can reach.

- [ ] Or install with the sample config:

```sh
curl -fsSL -o orion-agent-config.yaml \
  https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-config.yaml

curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- \
  --config ./orion-agent-config.yaml
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
sudo test -f /var/log/orion/agent.log
orion-agent logs --lines 20
```

macOS:

```sh
sudo test -f /usr/local/var/log/orion/agent.log
orion-agent logs --lines 20
```

- [ ] Confirm the Server appears in the Console Servers view.
- [ ] Confirm monitor rows appear after the first monitor interval.
- [ ] Restart the Server and confirm it does not create duplicate servers or monitors.

## Backup

- [ ] Run a SQLite backup after the first successful report:

```sh
docker compose -f orion-compose.yaml exec orion-core sqlite3 /data/orion.db ".backup '/data/orion-backup.db'"
docker cp "$(docker compose -f orion-compose.yaml ps -q orion-core):/data/orion-backup.db" ./orion-backup.db
```

- [ ] Store the backup somewhere outside the Docker volume.
