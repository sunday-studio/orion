# Orion scripts

Install, uninstall and test scripts for the Node. Users run the installer from a release; everything
else is for working on Orion. The step-by-step guide is
[Install and manage a Node](../../docs/deployment/node-install-upgrade.md).

## Shipped with every release

| Script | What it does |
|---|---|
| `node-bootstrap.sh` | Published as `orion-node-installer.sh`. Downloads the release binary and service files, verifies them against `SHA256SUMS`, installs, enrolls the host and runs `doctor`. `uninstall` as its first argument removes the Node. |
| `node-install.sh` | Installs the binary, config, state directory and service. Run by the bootstrap; also usable from a checkout (`sudo ./deploy/scripts/node-install.sh --core-url ...`). |
| `node-uninstall.sh` | Removes the service and binary; `--purge` also removes config, state and the service account. |

```sh
# install (the Console's Add server shows this with a one-time token filled in)
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh \
  | sudo ORION_ENROLLMENT_TOKEN='<token>' bash -s -- --core-url https://orion.example.com

# uninstall, keeping config and state; add --purge to remove them too
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh \
  | sudo bash -s -- uninstall --yes
```

## Checks and tools for contributors

| Script | What it does | Run with |
|---|---|---|
| `node-bootstrap-test.sh`, `test-node-bootstrap-verify.sh` | Installer behavior: plan output, missing Core URL, tampered or unlisted binaries, signatures, re-running. No privileges. | `make node-installer-test` |
| `node-cli-lifecycle-smoke.sh` | Dry-run of install and uninstall output. | `NODE_BINARY=<path> deploy/scripts/node-cli-lifecycle-smoke.sh` |
| `node-e2e-smoke.sh` | A real Core and the real Node on this machine: enroll, run, status, doctor, discover, maintenance, usage. | `make node-e2e` |
| `node-walkthrough.py` | Walks every CLI screen against the design board text. | `make node-walkthrough` |
| `node-perf-budget.py` | Checks the Node's CPU and memory budget. | `make node-perf` |
