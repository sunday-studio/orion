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

Publish the Core and Console image from GitHub Actions:

- workflow: `Core Image`;
- trigger: manual `workflow_dispatch`;
- required input: `version`, for example `v0.1.0`;
- optional input: `publish_latest`.

This produces:

```txt
ghcr.io/sunday-studio/orion-core:<version>
```

Use the same version tag in the sample Docker Compose file.

## Agent Binary

Publish Agent binaries from GitHub Actions:

- workflow: `Agent Binaries`;
- trigger: manual `workflow_dispatch`;
- required input: `version`, for example `v0.1.0`;
- optional input: `prerelease`.

This creates or updates the GitHub release and uploads:

```txt
orion-agent-linux-amd64
orion-agent-linux-arm64
orion-agent-darwin-amd64
orion-agent-darwin-arm64
```

The installer detects the host OS and architecture and downloads the matching asset from the latest
GitHub release unless `--version` is explicitly passed.

The Agent reports its baked version in system reports, so Core and Console can show which version
is installed.

## Compatibility

For the first deploy, Core and Agent should run the same release tag.

Patch releases in the same minor line are expected to stay wire-compatible. For example, `v0.1.1` Agent should work with `v0.1.0` Core unless release notes say otherwise.

Minor releases may add fields or behavior. Upgrade Core before Agents when moving across minor versions.
