# Build frontend (Vite + React)
FROM node:22-alpine AS frontend
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json frontend/
RUN cd frontend && npm ci
COPY frontend/ frontend/
RUN cd frontend && npm run build

# Build Go binary and embed frontend dist
FROM golang:1.25-alpine AS core
WORKDIR /app
COPY core/go.mod core/go.sum core/
RUN cd core && go mod download
COPY core/ core/
COPY --from=frontend /app/frontend/dist core/web
RUN cd core && go build -o orion-core .

# Minimal runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=core /app/core/orion-core .
COPY --from=core /app/core/web ./web
RUN mkdir -p /data
ENV ORION_DATA_DIR=/data
ENV ORION_PORT=8999
EXPOSE 8999
VOLUME /data
CMD ["./orion-core"]
