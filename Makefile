.PHONY: generate-openapi generate-sdk build-static docker-build docker-up seed-demo-data
generate-openapi:
	cd apps/core && ./scripts/generate-openapi.sh

generate-sdk: generate-openapi
	cd apps/console && npm run generate:api

build-static:
	cd apps/console && npm run build
	rm -rf apps/core/web
	mkdir -p apps/core/web
	cp -R apps/console/dist/. apps/core/web/

# Build orion-core Docker image (context: repo root)
docker-build:
	docker build -f apps/core/Dockerfile -t orion-core:latest .

# Run orion-core via docker compose (set ORION_ADMIN_* and ORION_JWT_SECRET for frontend auth)
docker-up:
	docker compose -f deploy/docker-compose.yml up -d

# Seed Core SQLite with 90 days of demo data for local UI/API testing
seed-demo-data:
	cd apps/core && go run ./scripts/seed-demo-data
