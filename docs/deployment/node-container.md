# Running the Node as a container

> **Not a supported install path yet.** The container Node is deferred (decision D-031). The image builds
> and runs, and this page records what was tested, but it has only been tried on a desktop Docker, not
> on a Linux server or a Kubernetes cluster, and it cannot see services, containers or pm2 apps. Install
> the Node on the host instead: [Install and manage a Node](node-install-upgrade.md).

The node is normally installed as a service on the host. It can also run as a container, which could suit hosts that run everything in Docker once this is supported.

The image is distroless (no shell, no package manager), runs as an unprivileged user (uid 65532), works with a read-only root filesystem and all capabilities dropped, and is about 17 MB. Configuration is environment variables only.

## Build

```sh
make node-image                 # ghcr.io/sunday-studio/orion-node:latest, built locally
make node-image VERSION=0.9.3   # tagged
```

## Run

Create a one-time token in the Console (Servers, Add server), then:

```sh
docker run -d --name orion-node --restart unless-stopped \
  --hostname "$(hostname)" \
  --read-only --cap-drop ALL --security-opt no-new-privileges \
  --pid=host \
  -v orion-node-data:/var/lib/orion \
  -e ORION_CORE_URL=https://orion.example.com \
  -e ORION_ENROLLMENT_TOKEN=<token> \
  ghcr.io/sunday-studio/orion-node:latest
```

The token is used once; after the first start the node keeps its own credentials in the `orion-node-data` volume, so the token can be removed from the command afterwards. Recreate the container without `ORION_ENROLLMENT_TOKEN` once it has enrolled: an environment variable stays visible in `docker inspect`. Keep the volume: removing it makes the node enroll again as a new server.

To run a node container beside Core on the same server, see
The Core compose override carries an experimental `node` profile (see the comment in `deploy/docker-compose.build.yml`). A standalone Compose version of the node is in [deploy/examples/node-container-compose.yaml](../../deploy/examples/node-container-compose.yaml).

## What it sees

| Setting | Effect |
|---|---|
| `--hostname` | The name the server gets in the Console. Without it, it is the container id. |
| `--pid=host` | The node sees the host's processes, so app CPU and memory (per process group) are the host's. Without it, only the container is visible. |
| capabilities dropped, non-root | `--pid=host` shows process names and CPU, but per-process disk I/O and details of other users' processes are limited by `/proc` permissions. |
| `-v /var/run/docker.sock:/var/run/docker.sock:ro --group-add <docker gid>` | Optional. Per-container CPU, memory, storage, Docker volumes, images and build cache reach the Console. The socket is root-equivalent on the host, which is why it is opt-in. |

## Limits compared with the host install

The container has no `systemctl`, `launchctl`, `pm2` or `docker` command, so:

- discovery does not suggest services, pm2 apps or containers (ports, disks, logs and storage still work);
- Console checks of type `systemd`, `docker` and `pm2` will fail on the container because the command they use is not there;
- `orion-node service ...` and `orion-node update` do not apply (`update` says so): replace the container with a newer image instead, and `doctor` skips the service check.

Process, port, disk and memory checks work. If you need service or container checks, use the host install.

## Kubernetes

A DaemonSet is not provided yet: the image runs in a pod with `hostPID: true`, but a manifest has not been tested on a cluster. See [Kubernetes position](kubernetes-position.md).

The image has a health check (`orion-node status --short`), so `docker ps` shows `healthy` while the node process is running and answering on its control socket. Whether it is connected to Core is shown by `docker exec orion-node orion-node status`.
