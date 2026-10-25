# Server and Node terminology

Decided 2026-05-29 (Server), updated 2026-10-21 (the agent became the Node).

## Decision

- **Server** is the product-facing word for a monitored machine: in the Console ("Servers",
  "Add server"), the README, the deployment guides and public screenshots.
- **Node** is the software that runs on a server and reports to Core: the `orion-node` command, the
  service, the release assets, the Docker image, the API (`/v1/nodes`) and the database (`nodes`,
  `node_id`). It was called the agent until 2026-10-21; see
  [Upgrade notes: the agent is now the node](../deployment/upgrade-notes-node-rename.md).

So a server runs a node, and a node reports for one server. Say "Server" when you mean the machine a
person looks at, and "Node" when you mean the program, its commands or its API.

## Rationale

`Server` matches how the intended self-hosted and homelab audience thinks about the machines Orion
monitors; it is clearer in navigation, incident context and installation docs than a technical noun.
"Agent" was dropped because it now mostly means an AI agent, and it is the most searched word in this
space. `Node` is short and neutral for the program itself.

## Scope

- Product-facing text says Server or Servers, unless it documents a literal command, path, API route,
  database field, config key, SDK symbol or release asset, which say `node`.
- Internal code uses `node` for the program and its API and `server` only in user-facing strings.
- Old names do not remain as aliases: routes under `/v1/agents` no longer exist and tokens stay valid
  because ids were kept.

## Related

- [Node/Core contract](../node-core-contract.md) documents the wire routes and ownership fields.
- [Install and manage a Node](../deployment/node-install-upgrade.md) documents `orion-node` and its paths.
- [Core features](core-features.md) lists the API routes.
- [Persistence and lifecycle](persistence-and-lifecycle.md) documents the SQLite schema names.
