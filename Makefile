.PHONY: generate-sdk build-static docker-build docker-up
generate-sdk:
	cd apps/web && npm run generate:api

# Build web source and copy dist to apps/core/web for SPA serving
build-static:
	cd apps/web && npm run build
	mkdir -p apps/core/web && cp -r apps/web/dist/* apps/core/web/

# Build orion-core Docker image (context: repo root)
docker-build:
	docker build -f apps/core/Dockerfile -t orion-core:latest .

# Run orion-core via docker compose (set ORION_ADMIN_* and ORION_JWT_SECRET for frontend auth)
docker-up:
	docker compose -f deploy/docker-compose.yml up -d
