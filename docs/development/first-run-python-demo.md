# First-Run Python Demo

Use this when you need a fast proof that Orion can watch a real local service, show a healthy
state, show a failure, and then show recovery in Console.

## What It Proves

- Core and Console start from Docker Compose.
- A tiny Python HTTP service exposes `/health`.
- The Orion Server CLI registers one Server and three monitors.
- `python-health` reports healthy, then failing, then healthy again.
- Console shows the Server, monitor list, monitor detail history, and incident/recovery path.

## Run It

From the repository root:

```sh
examples/python-sleep-compose/smoke.sh
```

The script builds and uses a local `orion-core:example-smoke` image by default, builds
`apps/agent/orion-agent` when needed, starts the example Compose stack with an isolated project name,
runs one healthy collection, toggles the Python app into failure, runs one failing collection,
removes the failure marker, and runs one recovery collection. Use `ORION_EXAMPLE_CORE_IMAGE` only
when intentionally testing a published or prebuilt Core image.

The smoke binds Core to `127.0.0.1:18999` and the Python app to `127.0.0.1:18080` by default, then
generates a temporary Server config under `tmp/` with those host ports.

Set `ORION_EXAMPLE_KEEP=1` to leave the containers and state database running for manual Console
inspection after the smoke completes.

## Manual Console Verification

Open `http://127.0.0.1:18999` and sign in with the example credentials:

- username: `admin`
- password: `change-me`

Check these Console views:

- Servers: `python-sleep-compose` appears once after the first Server run.
- Monitors: `python-health`, `python-port`, and `example-disk` appear.
- Monitor detail: `python-health` has recent reports; after the fail marker it shows a down report.
- Incidents: the failing `python-health` run opens an incident, and the recovery run resolves it.

## Reset

If you ran the smoke without `ORION_EXAMPLE_KEEP=1`, it cleans up containers, the Compose volume, and
the smoke state database on exit.

If you kept the stack for manual inspection:

```sh
cd examples/python-sleep-compose
docker compose -p orion-python-sleep-smoke down -v
rm -f ../../tmp/python-sleep-compose-smoke-state.db ../../tmp/python-sleep-compose-smoke-config.yaml
```
