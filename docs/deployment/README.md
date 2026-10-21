# Deployment Docs

The supported self-hosted setup is:

1. deploy Core and Console together with Docker;
2. install a Server on each monitored machine with the curlable `orion-node` installer;
3. point each Server at the Core URL it can reach.

- [Set up Core on a new server with Docker](docker-new-server.md)
- [First run checklist](first-run-checklist.md)
- [Core Docker deployment](core-docker.md)
- [Running the Node as a container](node-container.md)
- [Kubernetes position](kubernetes-position.md)
- [Server install and upgrade](node-install-upgrade.md)
- [Node configuration reference](node-config-reference.md)
- [Upgrade notes: the agent is now the node](upgrade-notes-node-rename.md)
- [Release readiness gate](release-readiness.md)
- [SQLite backup and restore](../sqlite-backup-restore.md)

Runtime examples live under `deploy/examples/`. The recommended first evaluation path is
[`examples/python-sleep-compose`](../../examples/python-sleep-compose/), which starts Core and
Console, monitors a Python `/health` endpoint, forces a failure, and verifies recovery. Maintainer
verification notes live in [First-run Python demo](../development/first-run-python-demo.md).

Maintainer release notes live in [Release packaging](release-packaging.md).
