.PHONY: core-soak core-soak-30m core-soak-72h generate-openapi generate-sdk console-sdk build-static docker-build docker-up docker-down docker-dev-up docker-dev-down console-dev docker-dev-seed agent-test core-test core-coverage core-race core-modernize-check core-vulncheck core-contract-check core-backend-verify console-build repository-smoke generated-contracts-check release-core-build release-readiness core-build core-worker-build agent-build seed-demo-data code-line-limit agent-build-all agent-installer-test agent-e2e agent-walkthrough agent-image agent-perf agent-theme agent-theme-check agent-config-reference

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
AGENT_CGO_ENABLED ?= 0

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

# Run Core tests with package/function coverage output.
core-coverage:
	cd apps/core && go test -coverprofile=$(CORE_COVERAGE_PROFILE) ./...
	cd apps/core && go tool cover -func=$(CORE_COVERAGE_PROFILE) | tee $(CORE_COVERAGE_SUMMARY)

# Run race detection on Core packages with scheduler, worker, and lifecycle concurrency.
core-soak:
	cd apps/core && go test ./internal/soak -run TestSoak -v -timeout 30m

# Cross-process soak: real API + Worker binaries, log ingest, live tails, heartbeats, log alerts.
core-soak-30m:
	cd apps/core && ORION_CROSS_PROCESS=1 ORION_CROSS_DURATION=30m ORION_CROSS_OUT=$${ORION_CROSS_OUT:-/tmp/orion-soak-30m.json} go test ./internal/soak -run TestCrossProcessLoad -count=1 -v -timeout 45m

core-soak-72h:
	cd apps/core && ORION_CROSS_PROCESS=1 ORION_CROSS_DURATION=72h ORION_CROSS_OUT=$${ORION_CROSS_OUT:-/tmp/orion-soak-72h.json} go test ./internal/soak -run TestCrossProcessLoad -count=1 -v -timeout 80h

core-race:
	cd apps/core && go test -race ./...

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
	bash deploy/scripts/test-agent-bootstrap-verify.sh
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
	docker build -f apps/core/Dockerfile --build-arg VERSION=$(VERSION) -t $(CORE_IMAGE):$(VERSION) .

# Build the Agent container image (context: repo root).
AGENT_IMAGE ?= ghcr.io/sunday-studio/orion-agent
agent-walkthrough:
	python3 deploy/scripts/agent-walkthrough.py

agent-image:
	docker build -f apps/agent/Dockerfile --build-arg VERSION=$(VERSION) -t $(AGENT_IMAGE):$(VERSION) .

# Build Core API for local/package validation.
core-build:
	cd apps/core && go build -trimpath -ldflags "-s -w -X orion/core/internal/buildinfo.Version=$(VERSION)" -o $(CORE_OUTPUT) ./cmd/api

# Build Core monitor worker for local/package validation.
core-worker-build:
	cd apps/core && go build -trimpath -ldflags "-s -w -X main.version=$(VERSION) -X orion/core/internal/buildinfo.Version=$(VERSION)" -o $(CORE_WORKER_OUTPUT) ./cmd/worker

# Build Orion Agent for the requested platform.
agent-build:
	cd apps/agent && CGO_ENABLED=$(AGENT_CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags "-s -w -X orion/agent/internal.Version=$(VERSION)" -o $(AGENT_OUTPUT) .

# Run orion-core via docker compose (set ORION_ADMIN_* and ORION_JWT_SECRET for frontend auth)
docker-up:
	ORION_CORE_IMAGE=$(CORE_IMAGE):$(VERSION) docker compose -f deploy/docker-compose.yml up -d

docker-down:
	docker compose -f deploy/docker-compose.yml down

# Start Core + Worker in Docker, then run the Console dev server (pnpm, hot reload) in the foreground.
# Console: http://localhost:5173  Core API: http://localhost:8999  (admin / local-admin-password-2026)
docker-dev-up:
	docker compose -f docker-compose.dev.yml up -d --build
	$(MAKE) console-dev

# Run only the Console dev server against an already-running Core.
console-dev:
	cd apps/console && { test -f .env || cp .env.example .env; } && pnpm install && pnpm generate:api && pnpm dev

docker-dev-down:
	docker compose -f docker-compose.dev.yml down

docker-dev-seed:
	docker compose -f docker-compose.dev.yml build orion-core-seed
	docker compose -f docker-compose.dev.yml stop orion-core-worker
	docker compose -f docker-compose.dev.yml run --rm orion-core-seed; status=$$?; \
		docker compose -f docker-compose.dev.yml start orion-core-worker; exit $$status

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

# Static agent builds for every supported target (Linux and macOS only, D-014).
agent-build-all:
	@for t in linux/amd64 linux/arm64 linux/arm/7 darwin/amd64 darwin/arm64; do \
	  os=$${t%%/*}; r=$${t#*/}; arch=$${r%%/*}; arm=; [ "$$arch" != "$$r" ] && arm=$${r#*/}; \
	  out=tmp/agent-dist/orion-agent-$$os-$$arch$${arm:+v$$arm}; mkdir -p tmp/agent-dist; \
	  (cd apps/agent && CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch GOARM=$$arm go build -trimpath -ldflags "-s -w -X orion/agent/internal.Version=$(VERSION)" -o ../../$$out .) || exit 1; \
	  echo "built $$out"; done

agent-installer-test:
	deploy/scripts/agent-bootstrap-test.sh

agent-e2e:
	deploy/scripts/agent-e2e-smoke.sh

agent-perf:
	cd apps/agent && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ../../tmp/orion-agent-perf .
	python3 deploy/scripts/agent-perf-budget.py tmp/orion-agent-perf

# CLI theme is generated from docs/design/tokens.json.
agent-theme:
	cd apps/agent && go generate ./internal/ui/

agent-theme-check: agent-theme
	git diff --exit-code apps/agent/internal/ui/theme_gen.go

agent-config-reference:
	cd apps/agent && go run ./internal/config/gen-reference -out ../../docs/deployment/agent-config-reference.md
