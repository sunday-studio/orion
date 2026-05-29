# Kubernetes Position

Orion's first supported deployment path is Docker Compose, not Kubernetes. The product is currently
optimized for small self-hosted installs where Core runs from a single persistent SQLite volume and
Servers push reports to it from the machines being monitored.

Kubernetes can make sense later when the operator already runs a cluster and wants Orion Core to be
managed with the same ingress, secret, backup, and upgrade workflow as other internal services. It
is not required for the first useful Orion setup, and it adds operational choices that Compose users
do not need to make.

## Current Recommendation

Use Docker Compose for the first install:

- one Core API and Console container;
- one persistent `/data` volume for SQLite and archives;
- one stable URL that monitored Servers can reach;
- one Server process installed on each monitored host.

If Core should stay available when the monitored home network is down, run Core on a small external
host or a Tailscale-reachable machine outside the failure domain being monitored.

## When Kubernetes Is Worth It

A Kubernetes deployment becomes useful when it proves something Compose does not:

- Core runs with persistent storage and a clear backup/restore path;
- ingress and TLS make the Core URL stable for monitored Servers;
- secrets are managed through Kubernetes primitives rather than `.env` files;
- upgrades preserve the SQLite volume and make rollback behavior explicit;
- monitored workloads or nodes can report to Core without weakening network boundaries.

## Minikube Example Criteria

Do not add a minikube example until it can demonstrate the full operational shape:

- a Core workload with persistent storage;
- an ingress or port-forwarded Core URL that Servers can use;
- a secret strategy for admin credentials and JWT signing;
- a monitored workload sending data to Core;
- notes for storage class selection, backups, ingress, TLS, and upgrades.

A Kubernetes example that only starts the web UI would be less useful than the Docker Compose path
and should wait.
