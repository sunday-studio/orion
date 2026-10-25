# Orion Node as a container. Build from the repository root:
#   docker build -f apps/node/Dockerfile -t orion-node .
FROM golang:1.26-alpine@sha256:5cb67f19c5e7994dcb80c074f7cfca8aeeada45bbdca035f73df269cefdcce61 AS build
WORKDIR /src
COPY apps/node/go.mod apps/node/go.sum apps/node/
RUN cd apps/node && go mod download
COPY apps/node/ apps/node/
ARG VERSION=dev
RUN cd apps/node && CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X orion/node/internal.Version=${VERSION}" -o /out/orion-node . \
    && mkdir -p /out/state

# Distroless static: CA certificates and timezone data, no shell and no package manager.
# nonroot is uid 65532. The root filesystem can be read-only; only /var/lib/orion is written.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/orion-node /usr/local/bin/orion-node
# A new named volume copies this directory's ownership, so the unprivileged user can write to it.
COPY --from=build --chown=65532:65532 /out/state /var/lib/orion
ENV ORION_CONTAINER=1 \
    ORION_STATE=/var/lib/orion/state.db \
    ORION_NODE_SOCKET=/var/lib/orion/node.sock \
    ORION_LOG_PATH=/var/lib/orion/node.log
VOLUME ["/var/lib/orion"]
USER nonroot
HEALTHCHECK --interval=60s --timeout=10s --start-period=60s --retries=3 \
    CMD ["/usr/local/bin/orion-node", "status", "--short"]
ENTRYPOINT ["/usr/local/bin/orion-node"]
CMD ["run"]
