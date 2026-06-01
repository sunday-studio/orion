# Release Packaging

Use one version for the Core image, Server binary, API contract, and Console build.

Use [Release readiness gate](release-readiness.md) before tagging a release candidate.

## Version Format

First deploy releases should use semantic version tags:

```txt
0.1.0
0.1.1
0.2.0
```

Pre-release builds can use a suffix:

```txt
0.1.0-rc.1
```

## Core Image

Publish the Core and Console image from GitHub Actions:

- workflow: `Core Image`;
- trigger: manual `workflow_dispatch`;
- required input: `version`, for example `0.1.0`;
- optional input: `publish_latest`.

This produces:

```txt
ghcr.io/sunday-studio/orion-core:<version>
```

Use the same version tag in the sample Docker Compose file.

## Server Binary

Publish Server binaries from GitHub Actions:

- workflow: `Server Binaries`;
- trigger: manual `workflow_dispatch`;
- required input: `version`, for example `0.1.0`;
- optional input: `prerelease`.

This creates or updates the GitHub release and uploads:

```txt
orion-agent-installer.sh
orion-agent-install.sh
orion-agent-uninstall.sh
orion-agent-systemd.service
orion-agent-config.yaml
com.orion.agent.plist
orion-agent-linux-amd64
orion-agent-linux-arm64
orion-agent-darwin-amd64
orion-agent-darwin-arm64
```

The installer detects the host OS and architecture and downloads the matching Server binary and
service assets from the latest GitHub release unless `--version` is explicitly passed.

The Server reports its baked version in system reports, so Core and Console can show which version
is installed.

## Compatibility

For the first deploy, Core and Server should run the same release tag.

Patch releases in the same minor line are expected to stay wire-compatible. For example, `0.1.1`
Server should work with `0.1.0` Core unless release notes say otherwise.

Minor releases may add fields or behavior. Upgrade Core before Servers when moving across minor versions.
