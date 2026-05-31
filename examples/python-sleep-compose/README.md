# Python Sleep Compose Example

This example runs a tiny Python service next to Orion Core. The Python process exposes `/health`
over HTTP and keeps a background worker alive with a `while True: sleep(...)` loop, which makes it a
small but real process for Orion to monitor.

The Orion Server still runs outside this Compose file through the `orion-agent` CLI. That mirrors the
normal deployment model: Core runs centrally, while each monitored machine runs its own Server
process and pushes reports to Core.

## Fast Smoke

From the repository root:

```sh
examples/python-sleep-compose/smoke.sh
```

The smoke builds and uses a local `orion-core:example-smoke` image by default, starts the Compose
stack, builds `apps/agent/orion-agent` when needed, runs one healthy Server collection, toggles the
Python app into failure, runs one failing collection, removes the failure marker, and verifies
recovery. Set `ORION_EXAMPLE_KEEP=1` to leave the stack running for Console inspection. Set
`ORION_EXAMPLE_CORE_IMAGE` only when you intentionally want to test a published or prebuilt Core
image.

With `ORION_EXAMPLE_KEEP=1`, open `http://127.0.0.1:18999` and sign in with `admin` / `change-me`.
The smoke uses isolated host ports `18999` and `18080` so it does not collide with a normal local
Core on `8999`.

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

Console verification path:

- Servers: `python-sleep-compose` should appear once.
- Monitors: `python-health`, `python-port`, and `example-disk` should appear.
- Monitor detail: `python-health` should show a recent up report.
- Incidents: no open incident is expected while the Python app is healthy.

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

Run the Server one more time after removing the marker. Console should show a healthy
`python-health` report again, and the incident created by the failing run should resolve.

## Reset

From `examples/python-sleep-compose`:

```sh
docker compose down -v
rm -f ../../tmp/python-sleep-compose-state.db
```

If you used `ORION_EXAMPLE_KEEP=1 examples/python-sleep-compose/smoke.sh`, reset its isolated
Compose project and state file from the repository root:

```sh
docker compose -p orion-python-sleep-smoke -f examples/python-sleep-compose/docker-compose.yaml down -v
rm -f tmp/python-sleep-compose-smoke-state.db tmp/python-sleep-compose-smoke-config.yaml
```
