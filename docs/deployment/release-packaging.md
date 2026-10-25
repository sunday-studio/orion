# Release Packaging

Use one version for the Core image, Node binary, API contract, and Console build.

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
- required input: `version`, which becomes the release tag, for example `v0.1.0` (installers and `orion-node update --version` use the same string);
- optional input: `publish_latest`.

This produces:

```txt
ghcr.io/sunday-studio/orion-core:<version>
```

Use the same version tag in the sample Docker Compose file.

## Node binary

Publish Node binaries from GitHub Actions:

- workflow: `Node Binaries`;
- trigger: manual `workflow_dispatch`;
- required input: `version`, which becomes the release tag, for example `v0.1.0` (installers and `orion-node update --version` use the same string);
- optional input: `prerelease`.

This creates or updates the GitHub release and uploads:

```txt
orion-node-installer.sh
orion-node-install.sh
orion-node-uninstall.sh
orion-node-systemd.service
orion-node-config.yaml
com.orion.node.plist
orion-node-linux-amd64
orion-node-linux-arm64
orion-node-darwin-amd64
orion-node-darwin-arm64
orion-node-sbom.spdx.json
SHA256SUMS
SHA256SUMS.sig
SHA256SUMS.pem
```

The installer detects the host OS and architecture and downloads the matching Node binary and
service assets from the latest GitHub release unless `--version` is explicitly passed.

The SPDX JSON SBOM covers the release asset directory. `SHA256SUMS` covers every executable and
installer asset the bootstrap script downloads; its keyless cosign signature and certificate let a
host with cosign verify the release workflow identity before installation.

The Node reports its baked version in system reports, so Core and Console can show which version
is installed.

## Compatibility

For the first deploy, Core and the Node should run the same release tag.

Patch releases in the same minor line are expected to stay wire-compatible. For example, a `0.1.1` Node
should work with a `0.1.0` Core unless release notes say otherwise.

Minor releases may add fields or behavior. Upgrade Core before Nodes when moving across minor versions.
