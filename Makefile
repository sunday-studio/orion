.PHONY: generate-openapi generate-sdk build-static docker-build docker-up docker-down core-build core-worker-build core-coverage agent-build seed-demo-data code-line-limit

VERSION ?= latest
CORE_IMAGE ?= ghcr.io/sunday-studio/orion-core
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
AGENT_OUTPUT ?= orion-agent
CORE_OUTPUT ?= orion-core
CORE_WORKER_OUTPUT ?= orion-core-worker
CORE_COVERAGE_PROFILE ?= /tmp/orion-core-coverage.out
CORE_COVERAGE_SUMMARY ?= /tmp/orion-core-coverage.txt
APP_CODE_LINE_LIMIT ?= 500
AGENT_CGO_ENABLED ?= 1

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

# Build Core API for local/package validation.
core-build:
	cd apps/core && go build -trimpath -ldflags "-s -w" -o $(CORE_OUTPUT) .

# Build Core monitor worker for local/package validation.
core-worker-build:
	cd apps/core && go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(CORE_WORKER_OUTPUT) ./cmd/worker

# Run Core tests with package/function coverage output.
core-coverage:
	cd apps/core && go test -coverprofile=$(CORE_COVERAGE_PROFILE) ./...
	cd apps/core && go tool cover -func=$(CORE_COVERAGE_PROFILE) | tee $(CORE_COVERAGE_SUMMARY)

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

# Enforce the app source file line limit documented in AGENTS.md.
code-line-limit:
	@violations=$$(find apps -type f \( -name '*.go' -o -name '*.ts' -o -name '*.tsx' -o -name '*.js' -o -name '*.jsx' -o -name '*.mjs' -o -name '*.css' -o -name '*.sh' \) \
		-not -path '*/node_modules/*' \
		-not -path '*/dist/*' \
		-not -path '*/web/*' \
		-not -path '*/docs/*' \
		-not -path '*/orion-sdk/*' \
		-not -path '*/db/migrations/*' \
		-not -path '*/public/*' \
		-not -name '*.config.ts' \
		-not -name '*.config.js' \
		-not -name '*.config.mjs' \
		-not -name '*.config.cjs' \
		-not -name '*.d.ts' \
		-print0 | xargs -0 wc -l | awk -v max="$(APP_CODE_LINE_LIMIT)" '$$2 != "total" && $$1 > max { printf "%5d %s\n", $$1, $$2 }' | sort -nr); \
	if [ -n "$$violations" ]; then \
		printf 'code-line-limit: app source files exceed %s lines\n' "$(APP_CODE_LINE_LIMIT)" >&2; \
		printf '%s\n' "$$violations" >&2; \
		exit 1; \
	fi; \
	printf 'code-line-limit: all app source files are at or below %s lines\n' "$(APP_CODE_LINE_LIMIT)"
