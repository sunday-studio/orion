# First Run Checklist

Use this checklist for a first self-hosted Orion run with one Core and one Agent.

## Core

- [ ] Export the required Console auth environment variables:

```sh
export ORION_ADMIN_USERNAME=admin
export ORION_ADMIN_PASSWORD='change-me'
export ORION_JWT_SECRET='change-me-to-a-long-random-value'
```

- [ ] Build the Core image:

```sh
make docker-build
```

- [ ] Start Core:

```sh
make docker-up
```

- [ ] Check health:

```sh
curl http://localhost:8999/health
```

- [ ] Open `http://localhost:8999` and sign in with the configured admin credentials.
- [ ] Confirm the `orion-data` Docker volume exists and is backed up.

## Agent

- [ ] Build the Agent binary:

```sh
cd apps/agent
go build -o orion-agent .
cd ../..
```

- [ ] Install the Agent with a Core URL reachable from the monitored machine:

```sh
sudo ./deploy/scripts/agent-install.sh \
  --core-url http://orion-core.local:8999 \
  --binary ./apps/agent/orion-agent
```

- [ ] Confirm the Agent service is running.
- [ ] Confirm local Agent state exists:

Linux:

```sh
sudo test -f /var/lib/orion/state.db
```

macOS:

```sh
sudo test -f /usr/local/var/lib/orion/state.db
```

- [ ] Confirm the Agent appears in the Console Agents view.
- [ ] Confirm monitor rows appear after the first monitor interval.
- [ ] Restart the Agent and confirm it does not create duplicate agents or monitors.

## Backup

- [ ] Run a SQLite backup after the first successful report:

```sh
docker compose -f deploy/docker-compose.yml exec orion-core sqlite3 /data/orion.db ".backup '/data/orion-backup.db'"
docker cp "$(docker compose -f deploy/docker-compose.yml ps -q orion-core):/data/orion-backup.db" ./orion-backup.db
```

- [ ] Store the backup somewhere outside the Docker volume.
