# Upgrade notes: the agent is now the node

The software that runs on each monitored machine, called the agent, is now called the **node**. This
is a breaking change for anything that names the old thing: servers must be re-installed once, and
scripts that call the API or the command line need new names. Nothing is lost: your servers, monitors,
incidents, history and settings carry over, and a server keeps the same identity and token.

## Upgrade in this order

1. **Upgrade Core.** Pull or rebuild the new image and restart. Core copies the database to
   `/data/backups` first, then renames its tables and stored names in one migration. Servers that
   still run the old agent stop being served until step 2, so do both in one sitting.
2. **Re-run the installer on every server.** It is the same command as before; the Console's
   **Servers, Add server** shows it, and it now downloads `orion-node`:

   ```sh
   curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh \
     | sudo ORION_ENROLLMENT_TOKEN='<token>' bash -s -- --core-url https://orion.example.com
   ```

   The installer stops and removes the old `orion-agent` service and binary, installs `orion-node`,
   and keeps `/etc/orion/config.yaml`, the state database and the `orion` service user. The node finds
   its existing registration, so it does not enrol again.
3. **Old agents cannot update themselves.** Their `orion-agent update` looks for assets that are no
   longer published. Use the installer.

Running the agent in Docker? Pull or build `orion-node` instead of `orion-agent`, keep the same data
volume and run it with the same settings.

## What changed

| Was | Is |
|---|---|
| `orion-agent` command, binary and package | `orion-node` |
| `orion-agent-installer.sh`, release assets `orion-agent-<os>-<arch>` | `orion-node-installer.sh`, `orion-node-<os>-<arch>` |
| systemd `orion-agent.service`, launchd `com.orion.agent` | `orion-node.service`, `com.orion.node` |
| Docker image `orion-agent` | `orion-node` |
| `ORION_AGENT_*` environment variables | `ORION_NODE_*` |
| API `/v1/agents/...`, fields `agent_id`, `agent_*` | `/v1/nodes/...`, `node_id`, `node_*` |
| `orion-agent ... --json` key `agent_id` | `node_id`; `schema_version` is now 3 |
| Make targets `agent-*` | `node-*` |

The paths `/etc/orion`, `/var/lib/orion`, the `orion` service user and the `ORION_` prefix are unchanged.
In the Console a monitored machine is still a **Server**; the **node** is the software on it.

## For people who call the API

Replace `/v1/agents` with `/v1/nodes`, and `agent_id`, `agent_ids`, `agent_version` and similar
fields with their `node_` names. Audit log actions and event kinds that began with `agent_` or
`agent.` now begin with `node_` or `node.`, including the ones already recorded. The OpenAPI
contract (`apps/core/openapi.yaml`) is the full list.
