# Deployment Docs

The supported self-hosted setup is:

1. deploy Core and Console together with Docker;
2. install a Server on each monitored machine with the curlable `orion-agent` installer;
3. point each Server at the Core URL it can reach.

- [First run checklist](first-run-checklist.md)
- [Core Docker deployment](core-docker.md)
- [Kubernetes position](kubernetes-position.md)
- [Server install and upgrade](agent-install-upgrade.md)
- [SQLite backup and restore](../sqlite-backup-restore.md)

Runtime examples live under `deploy/examples/`.

Maintainer release notes live in [Release packaging](release-packaging.md).
