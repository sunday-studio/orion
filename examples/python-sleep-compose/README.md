# Python Sleep Compose Example

This example runs a tiny Python service next to Orion Core. The Python process exposes `/health`
over HTTP and keeps a background worker alive with a `while True: sleep(...)` loop, which makes it a
small but real process for Orion to monitor.

The Orion Server still runs outside this Compose file through the `orion-agent` CLI. That mirrors the
normal deployment model: Core runs centrally, while each monitored machine runs its own Server
process and pushes reports to Core.

## Start Core and the Python App

```sh
cd examples/python-sleep-compose
docker compose up -d --build
curl http://localhost:8080/health
curl http://localhost:8999/health
```

## Run the Orion Server Once

From the repository root, build the compatibility CLI if you do not already have `orion-agent`
installed:

```sh
make agent-build VERSION=example
```

Run one foreground collection cycle against the example config:

```sh
apps/agent/orion-agent \
  -config examples/python-sleep-compose/server-config.yaml \
  -state tmp/python-sleep-compose-state.db \
  run -once -verbose
```

Open `http://localhost:8999` and sign in with:

- username: `admin`
- password: `change-me`

You should see one Server and monitor results for:

- `python-health`: HTTP `/health` check;
- `python-port`: TCP reachability on port `8080`;
- `example-disk`: local disk threshold from the machine running the Server.

## Make the App Unhealthy

Create a fail marker inside the Python container:

```sh
docker compose exec python-app sh -c 'touch /tmp/orion-example-fail'
```

Run the Server again:

```sh
apps/agent/orion-agent \
  -config examples/python-sleep-compose/server-config.yaml \
  -state tmp/python-sleep-compose-state.db \
  run -once -verbose
```

The `python-health` monitor should report a failed HTTP status. Remove the marker to recover:

```sh
docker compose exec python-app sh -c 'rm -f /tmp/orion-example-fail'
```

## Reset

```sh
docker compose down -v
rm -f tmp/python-sleep-compose-state.db
```
