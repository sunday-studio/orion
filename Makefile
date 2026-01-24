.PHONY: generate-sdk build-static docker-build docker-up
generate-sdk:
	cd frontend && npm run generate:api

# Build frontend and copy to core/web for SPA serving
build-static:
	cd frontend && npm run build
	mkdir -p core/web && cp -r frontend/dist/* core/web/

# Build orion-core Docker image (context: repo root)
docker-build:
	docker build -f core/Dockerfile -t orion-core:latest .

# Run orion-core via docker compose (set ORION_ADMIN_* and ORION_JWT_SECRET for frontend auth)
docker-up:
	docker compose up -d
