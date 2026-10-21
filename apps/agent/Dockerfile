# Orion Agent as a container. Build from the repository root:
#   docker build -f apps/agent/Dockerfile -t orion-agent .
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY apps/agent/go.mod apps/agent/go.sum apps/agent/
RUN cd apps/agent && go mod download
COPY apps/agent/ apps/agent/
ARG VERSION=dev
RUN cd apps/agent && CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X orion/agent/internal.Version=${VERSION}" -o /out/orion-agent . \
    && mkdir -p /out/state

# Distroless static: CA certificates and timezone data, no shell and no package manager.
# nonroot is uid 65532. The root filesystem can be read-only; only /var/lib/orion is written.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/orion-agent /usr/local/bin/orion-agent
# A new named volume copies this directory's ownership, so the unprivileged user can write to it.
COPY --from=build --chown=65532:65532 /out/state /var/lib/orion
ENV ORION_CONTAINER=1 \
    ORION_STATE=/var/lib/orion/state.db \
    ORION_AGENT_SOCKET=/var/lib/orion/agent.sock \
    ORION_LOG_PATH=/var/lib/orion/agent.log
VOLUME ["/var/lib/orion"]
USER nonroot
ENTRYPOINT ["/usr/local/bin/orion-agent"]
CMD ["run"]
