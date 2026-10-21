# Running the Agent as a container

The agent is normally installed as a service on the host (see [Server install and upgrade](agent-install-upgrade.md)). It can also run as a container, which suits hosts that run everything in Docker.

The image is distroless (no shell, no package manager), runs as an unprivileged user (uid 65532), works with a read-only root filesystem and all capabilities dropped, and is about 17 MB. Configuration is environment variables only.

## Build

```sh
make agent-image                 # ghcr.io/sunday-studio/orion-agent:latest, built locally
make agent-image VERSION=0.9.3   # tagged
```

## Run

Create a one-time token in the Console (Servers, Add server), then:

```sh
docker run -d --name orion-agent --restart unless-stopped \
  --hostname "$(hostname)" \
  --read-only --cap-drop ALL --security-opt no-new-privileges \
  --pid=host \
  -v orion-agent-data:/var/lib/orion \
  -e ORION_CORE_URL=https://orion.example.com \
  -e ORION_ENROLLMENT_TOKEN=<token> \
  ghcr.io/sunday-studio/orion-agent:latest
```

The token is used once; after the first start the agent keeps its own credentials in the `orion-agent-data` volume, so the token can be removed from the command afterwards. Recreate the container without `ORION_ENROLLMENT_TOKEN` once it has enrolled: an environment variable stays visible in `docker inspect`. Keep the volume: removing it makes the agent enroll again as a new server.

A Compose version is in [deploy/examples/agent-container-compose.yaml](../../deploy/examples/agent-container-compose.yaml).

## What it sees

| Setting | Effect |
|---|---|
| `--hostname` | The name the server gets in the Console. Without it, it is the container id. |
| `--pid=host` | The agent sees the host's processes, so app CPU and memory (per process group) are the host's. Without it, only the container is visible. |
| capabilities dropped, non-root | `--pid=host` shows process names and CPU, but per-process disk I/O and details of other users' processes are limited by `/proc` permissions. |
| `-v /var/run/docker.sock:/var/run/docker.sock:ro --group-add <docker gid>` | Optional. Per-container CPU, memory, storage, Docker volumes, images and build cache reach the Console. The socket is root-equivalent on the host, which is why it is opt-in. |

## Limits compared with the host install

The container has no `systemctl`, `launchctl`, `pm2` or `docker` command, so:

- discovery does not suggest services, pm2 apps or containers (ports, disks, logs and storage still work);
- Console checks of type `systemd`, `docker` and `pm2` will fail on the container because the command they use is not there;
- `orion-agent service ...` and `orion-agent update` do not apply (`update` says so): replace the container with a newer image instead, and `doctor` skips the service check.

Process, port, disk and memory checks work. If you need service or container checks, use the host install.

## Kubernetes

A DaemonSet is not provided yet: the image runs in a pod with `hostPID: true`, but a manifest has not been tested on a cluster. See [Kubernetes position](kubernetes-position.md).

The image has a health check (`orion-agent status --short`), so `docker ps` shows `healthy` once the agent is connected to Core.
