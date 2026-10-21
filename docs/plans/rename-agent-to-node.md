# Rename "agent" to "node"

Decided 2026-10-21: the Orion software that runs on a monitored machine is renamed from **agent** to
**node**, everywhere: binary, package, Docker image, installer, service, API, database, Console, docs and
design. "Agent" collides with AI agents, and it is the most searched word in this space.

Scope was chosen as everything, including the Core API and database. That is a breaking change to the
wire contract: a Core with the new API does not serve an already-installed agent, so every server re-runs
the installer after Core is upgraded. There are no compatibility routes.

## What is renamed

| Was | Becomes |
|---|---|
| `apps/agent/`, Go module `orion/agent` | `apps/node/`, `orion/node` |
| `orion-agent` binary and CLI, `orion-agent-installer.sh`, release assets `orion-agent-<os>-<arch>` | `orion-node`, `orion-node-installer.sh`, `orion-node-<os>-<arch>` |
| Docker image `ghcr.io/sunday-studio/orion-agent` | `ghcr.io/sunday-studio/orion-node` |
| systemd unit `orion-agent.service`, launchd `com.orion.agent` | `orion-node.service`, `com.orion.node` |
| `ORION_AGENT_*` environment variables (socket, hostname, image, bin, config) | `ORION_NODE_*` |
| API `/v1/agents/...`, JSON `agent_id`, `agent_*` fields | `/v1/nodes/...`, `node_id`, `node_*` |
| Tables `agents`, `agent_*` and `agent_id` columns | `nodes`, `node_*`, `node_id` (one migration renames them, data is kept) |
| Go types and files named `Agent...`, `agent-*.go` | `Node...`, `node-*.go` |
| "agent" in Console, docs, website, CLI text and design boards | "node" |

## What is not renamed

- `AGENTS.md`, `.agents/`, `COORDINATION.md` wording about coding agents: those mean AI agents.
- HTTP `User-Agent` headers and any "user agent" text.
- Old migration files and old release notes (history stays as it was). Old audit log rows keep their
  `agent_*` action names; new rows use `node_*`.
- `/etc/orion`, `/var/lib/orion`, the `orion` service user and the `ORION_` prefix.

## Open wording question

The Console calls a monitored machine a **Server** ("Servers, Add server"). The software on it becomes
**Node**. The recommendation is to keep "Server" in the Console for the machine and use "Node" for the
software (`orion-node`, "the Node is not running"). orion:design to confirm.

## Order and owners

1. **Core and Node, one series** (orion:agent session): API, database migration, Go packages, binary,
   installer, workflows, Docker, Makefile, contract and deployment docs. Regenerate OpenAPI and the SDK.
2. **Console** (orion:web): move to the regenerated SDK and the new routes, copy changes. It cannot land
   before step 1 nor be left behind, so steps 1 and 2 are pushed together.
3. **Design boards and website** (orion:design, then the website docs): CLI boards, Servers pages, copy.
4. **Verification:** the full test suites, `make agent-e2e` (renamed), the Docker images and
   `make agent-walkthrough` on the renamed binary, then the release.

Every step goes through the independent review before it is pushed. Because steps 1 and 2 change the
wire contract together, they are pushed as one series.
