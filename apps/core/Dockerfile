# Build frontend (Vite + React)
FROM --platform=$BUILDPLATFORM node:22-alpine@sha256:0a7108bf6c7bf5de370ffb1a3ed6be93d405b43ff159f681a8d18c0e2bc2e402 AS frontend
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
FROM golang:1.27-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS core
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
