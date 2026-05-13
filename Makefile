.PHONY: generate-openapi generate-sdk build-static docker-build docker-up
generate-openapi:
	cd apps/core && ./scripts/generate-openapi.sh

generate-sdk: generate-openapi
	cd apps/console && npm run generate:api

# Build console source and copy dist to apps/core/web for SPA serving
build-static:
	cd apps/console && npm run build
	mkdir -p apps/core/web && cp -r apps/console/dist/* apps/core/web/

# Build orion-core Docker image (context: repo root)
docker-build:
	docker build -f apps/core/Dockerfile -t orion-core:latest .

# Run orion-core via docker compose (set ORION_ADMIN_* and ORION_JWT_SECRET for frontend auth)
docker-up:
	docker compose -f deploy/docker-compose.yml up -d
