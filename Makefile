.PHONY: generate-openapi generate-sdk console-sdk build-static docker-build docker-up docker-down agent-test core-test core-race core-modernize-check core-vulncheck core-contract-check core-backend-verify console-build repository-smoke generated-contracts-check release-core-build release-readiness core-build core-worker-build agent-build seed-demo-data

VERSION ?= latest
CORE_IMAGE ?= ghcr.io/sunday-studio/orion-core
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
AGENT_OUTPUT ?= orion-agent
CORE_OUTPUT ?= orion-core
CORE_WORKER_OUTPUT ?= orion-core-worker
AGENT_CGO_ENABLED ?= 1

generate-openapi:
	cd apps/core && ./scripts/generate-openapi.sh

generate-sdk: generate-openapi
	cd apps/console && pnpm run generate:api

console-sdk:
	cd apps/console && pnpm run generate:api

build-static: console-sdk
	cd apps/console && pnpm run build
	rm -rf apps/core/web
	mkdir -p apps/core/web
	cp -R apps/console/dist/. apps/core/web/

# Run the full Core Go test suite.
core-test:
	cd apps/core && go test ./...

# Run race detection on Core packages with scheduler, worker, and lifecycle concurrency.
core-race:
	cd apps/core && go test -race ./internal/service ./internal/worker

# Run the Core Go modernization lint gate. Requires golangci-lint v2.6 or newer.
core-modernize-check:
	cd apps/core && golangci-lint run --config ../../.golangci.yml --new-from-merge-base=main ./...

# Run the Core Go vulnerability gate. Requires govulncheck.
core-vulncheck:
	cd apps/core && govulncheck ./...

# Regenerate Core OpenAPI output and fail if generated contract files drift.
core-contract-check: generate-openapi
	git diff --exit-code -- apps/core/docs apps/core/openapi.yaml

# Local Core backend verification bundle used before opening backend PRs.
core-backend-verify: core-test core-race core-modernize-check core-vulncheck core-contract-check core-build core-worker-build

agent-test:
	cd apps/agent && go test ./...

console-build: console-sdk
	cd apps/console && pnpm run build

repository-smoke:
	bash -n deploy/scripts/*.sh
	ORION_ADMIN_USERNAME=ci-admin ORION_ADMIN_PASSWORD=ci-password ORION_JWT_SECRET=ci-secret-at-least-long-enough docker compose -f deploy/docker-compose.yml config >/tmp/orion-compose.yml

generated-contracts-check: generate-sdk
	git diff --exit-code -- apps/core/docs apps/core/openapi.yaml apps/console/src/orion-sdk
	test -s apps/console/src/orion-sdk/index.ts

release-core-build:
	$(MAKE) core-build CORE_OUTPUT=/tmp/orion-core
	$(MAKE) core-worker-build CORE_WORKER_OUTPUT=/tmp/orion-core-worker

release-readiness: agent-test core-test release-core-build console-build repository-smoke

# Build orion-core Docker image (context: repo root)
docker-build:
	docker build -f apps/core/Dockerfile -t $(CORE_IMAGE):$(VERSION) .

# Build Core API for local/package validation.
core-build:
	cd apps/core && go build -trimpath -ldflags "-s -w" -o $(CORE_OUTPUT) .

# Build Core monitor worker for local/package validation.
core-worker-build:
	cd apps/core && go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(CORE_WORKER_OUTPUT) ./cmd/worker

# Build Orion Agent for the requested platform.
agent-build:
	cd apps/agent && CGO_ENABLED=$(AGENT_CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags "-s -w -X orion/agent/internal.Version=$(VERSION)" -o $(AGENT_OUTPUT) .

# Run orion-core via docker compose (set ORION_ADMIN_* and ORION_JWT_SECRET for frontend auth)
docker-up:
	ORION_CORE_IMAGE=$(CORE_IMAGE):$(VERSION) docker compose -f deploy/docker-compose.yml up -d

docker-down:
	docker compose -f deploy/docker-compose.yml down

# Seed Core SQLite with 90 days of demo data for local UI/API testing
seed-demo-data:
	cd apps/core && go run ./scripts/seed-demo-data
