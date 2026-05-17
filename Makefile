.PHONY: generate-openapi generate-sdk build-static docker-build docker-up docker-down agent-build seed-demo-data

VERSION ?= latest
CORE_IMAGE ?= ghcr.io/sunday-studio/orion-core
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
AGENT_OUTPUT ?= orion-agent

generate-openapi:
	cd apps/core && ./scripts/generate-openapi.sh

generate-sdk: generate-openapi
	cd apps/console && pnpm run generate:api

build-static:
	cd apps/console && pnpm run build
	rm -rf apps/core/web
	mkdir -p apps/core/web
	cp -R apps/console/dist/. apps/core/web/

# Build orion-core Docker image (context: repo root)
docker-build:
	docker build -f apps/core/Dockerfile -t $(CORE_IMAGE):$(VERSION) .

# Build Orion Agent for the requested platform.
agent-build:
	cd apps/agent && CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags "-s -w -X orion/agent/internal.Version=$(VERSION)" -o $(AGENT_OUTPUT) .

# Run orion-core via docker compose (set ORION_ADMIN_* and ORION_JWT_SECRET for frontend auth)
docker-up:
	ORION_CORE_IMAGE=$(CORE_IMAGE):$(VERSION) docker compose -f deploy/docker-compose.yml up -d

docker-down:
	docker compose -f deploy/docker-compose.yml down

# Seed Core SQLite with 90 days of demo data for local UI/API testing
seed-demo-data:
	cd apps/core && go run ./scripts/seed-demo-data
