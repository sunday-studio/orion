# Server Terminology Decision

Date: 2026-05-29

## Decision

Use **Server** as the product-facing term for a monitored machine in the Console, README, deployment
guides, and public evaluation screenshots.

Keep **node** as compatibility terminology for existing implementation and integration surfaces:

- API routes and route parameters such as `/v1/nodes`, `/v1/nodes/:node_id/report`, and
  `/v1/nodes/:node_id/token/status`;
- generated SDK symbols such as `useGetNodes`, `ApiNodeResponse`, and node query parameter names;
- SQLite tables and fields such as `nodes`, `node_id`, `node_name`, `owner_kind = node`, and
  `runner = node`;
- installed binary, service, release assets, and helper scripts such as `orion-node`,
  `orion-node-installer.sh`, `orion-node.service`, and `com.orion.node.plist`;
- config, state, and log field names that already ship with the Server runtime.

## Rationale

`Server` matches how the intended self-hosted and homelab audience thinks about the machines Orion
monitors. It is clearer in navigation, incident context, monitor ownership, and installation docs
than exposing `Node` as a primary product noun.

Renaming compatibility surfaces now would create avoidable breakage for existing installs, scripts,
API consumers, generated clients, database migrations, and release assets. Those names can only move
behind an explicit compatibility plan with redirects, aliases, migration tests, and release notes.

## Scope

Product-facing surfaces should say Server or Servers unless they are documenting a literal command,
path, API route, database field, config key, generated SDK symbol, or release asset.

Internal code may continue using `node` names where it maps directly to the existing API contract
or database schema. New internal names should prefer `server` only when they do not obscure the
contract boundary or force churn in generated code.

## Linked Areas Intentionally Left As Node

- [Node/Core contract](../node-core-contract.md) documents wire routes and ownership fields.
- [Server install and upgrade](../deployment/node-install-upgrade.md) documents `orion-node`
  binary and service paths.
- [Core features](core-features.md) lists existing API routes.
- [Persistence and lifecycle](persistence-and-lifecycle.md) documents SQLite schema names.
