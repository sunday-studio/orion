# Build frontend (Vite + React)
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
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
FROM golang:1.25-alpine AS core
WORKDIR /app
RUN apk add --no-cache build-base
COPY apps/core/go.mod apps/core/go.sum apps/core/
RUN cd apps/core && go mod download
COPY apps/core/ apps/core/
COPY --from=frontend /app/apps/console/dist apps/core/web
RUN cd apps/core && go build -o orion-core .
RUN cd apps/core && go build -o orion-core-worker ./cmd/worker

# Minimal runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates sqlite
WORKDIR /app
COPY --from=core /app/apps/core/orion-core .
COPY --from=core /app/apps/core/orion-core-worker .
COPY --from=core /app/apps/core/web ./web
RUN mkdir -p /data
ENV ORION_DATA_DIR=/data
ENV ORION_PORT=8999
EXPOSE 8999
VOLUME /data
CMD ["./orion-core"]
