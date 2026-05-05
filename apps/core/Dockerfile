# Build frontend (Vite + React)
FROM node:22-alpine AS frontend
WORKDIR /app
COPY apps/web/package.json apps/web/package-lock.json apps/web/
RUN cd apps/web && npm ci
COPY apps/web/ apps/web/
RUN cd apps/web && npm run build

# Build Go binary and embed frontend dist
FROM golang:1.25-alpine AS core
WORKDIR /app
COPY apps/core/go.mod apps/core/go.sum apps/core/
RUN cd apps/core && go mod download
COPY apps/core/ apps/core/
COPY --from=frontend /app/apps/web/dist apps/core/web
RUN cd apps/core && go build -o orion-core .

# Minimal runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=core /app/apps/core/orion-core .
COPY --from=core /app/apps/core/web ./web
RUN mkdir -p /data
ENV ORION_DATA_DIR=/data
ENV ORION_PORT=8999
EXPOSE 8999
VOLUME /data
CMD ["./orion-core"]
