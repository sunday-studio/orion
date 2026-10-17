# Build frontend (Vite + React)
FROM --platform=$BUILDPLATFORM node:25-alpine@sha256:bdf2cca6fe3dabd014ea60163eca3f0f7015fbd5c7ee1b0e9ccb4ced6eb02ef4 AS frontend
WORKDIR /app
ARG VITE_API_BASE_URL=/v1
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
RUN corepack enable && corepack prepare pnpm@10.23.0 --activate
COPY apps/console/package.json apps/console/pnpm-lock.yaml apps/console/
RUN cd apps/console && pnpm install --frozen-lockfile
COPY apps/core/openapi.yaml apps/core/openapi.yaml
COPY apps/console/ apps/console/
RUN cd apps/console && pnpm generate:api
RUN cd apps/console && pnpm run build

# Build Go binary and carry frontend dist for runtime serving
FROM golang:1.25-alpine@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59 AS core
WORKDIR /app
RUN apk add --no-cache build-base
COPY apps/core/go.mod apps/core/go.sum apps/core/
RUN cd apps/core && go mod download
COPY apps/core/ apps/core/
COPY --from=frontend /app/apps/console/dist apps/core/web
RUN cd apps/core && go build -o orion-core ./cmd/api
RUN cd apps/core && go build -o orion-core-worker ./cmd/worker

# Minimal runtime. Starts as root only long enough to fix ownership of an existing
# /data volume (see docker-entrypoint.sh), then drops to the unprivileged orion user.
FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc
RUN apk add --no-cache ca-certificates sqlite su-exec \
    && addgroup -S -g 10001 orion \
    && adduser -S -u 10001 -G orion -h /app orion
WORKDIR /app
COPY --from=core /app/apps/core/orion-core .
COPY --from=core /app/apps/core/orion-core-worker .
COPY --from=core /app/apps/core/web ./web
COPY apps/core/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod 0755 /usr/local/bin/docker-entrypoint.sh \
    && mkdir -p /data && chown orion:orion /data && chmod 0700 /data
ENV ORION_DATA_DIR=/data
ENV ORION_PORT=8999
ENV GIN_MODE=release
EXPOSE 8999
VOLUME /data
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["./orion-core"]
