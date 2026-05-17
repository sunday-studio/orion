# Release Packaging

Use one version for the Core image, Agent binary, API contract, and Console build.

## Version Format

First deploy releases should use semantic version tags:

```txt
v0.1.0
v0.1.1
v0.2.0
```

Pre-release builds can use a suffix:

```txt
v0.1.0-rc.1
```

## Core Image

Build a tagged Core image from the repository root:

```sh
CORE_IMAGE=ghcr.io/sunday-studio/orion-core VERSION=v0.1.0 make docker-build
```

This produces:

```txt
ghcr.io/sunday-studio/orion-core:v0.1.0
```

Use a local image name for local-only testing:

```sh
CORE_IMAGE=orion-core VERSION=v0.1.0 make docker-build
```

## Agent Binary

Build an Agent binary from the repository root:

```sh
VERSION=v0.1.0 make agent-build
```

Cross-build by setting `GOOS` and `GOARCH`:

```sh
VERSION=v0.1.0 GOOS=linux GOARCH=amd64 AGENT_OUTPUT=orion-agent-linux-amd64 make agent-build
VERSION=v0.1.0 GOOS=darwin GOARCH=arm64 AGENT_OUTPUT=orion-agent-darwin-arm64 make agent-build
```

The Agent reports this version in its system reports, so Core and Console can show which version is installed.

## Compatibility

For the first deploy, Core and Agent should run the same release tag.

Patch releases in the same minor line are expected to stay wire-compatible. For example, `v0.1.1` Agent should work with `v0.1.0` Core unless release notes say otherwise.

Minor releases may add fields or behavior. Upgrade Core before Agents when moving across minor versions.
