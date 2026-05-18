# Deployment Docs

The supported self-hosted setup is:

1. deploy Core and Console together with Docker;
2. install an Agent on each monitored machine with the curlable installer;
3. point each Agent at the Core URL it can reach.

- [First run checklist](first-run-checklist.md)
- [Core Docker deployment](core-docker.md)
- [Agent install and upgrade](agent-install-upgrade.md)
- [SQLite backup and restore](../sqlite-backup-restore.md)

Runtime examples live under `deploy/examples/`.

Maintainer release notes live in [Release packaging](release-packaging.md).
