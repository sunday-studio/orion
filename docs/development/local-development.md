# Local development

Run the bundled Core and Console example from this repository:

```sh
cd deploy/examples
ADMIN_PASSWORD="$(openssl rand -base64 24)"
JWT_SECRET="$(openssl rand -base64 32)"
cat > .env <<EOF
ORION_REQUIRE_FRONTEND_AUTH=true
ORION_ADMIN_USERNAME=admin
ORION_ADMIN_PASSWORD=$ADMIN_PASSWORD
ORION_JWT_SECRET=$JWT_SECRET
EOF
docker compose -f ./core-console-compose.yaml up -d
curl http://localhost:8999/health
```

Work on the Console against a local Core:

```sh
cd apps/console
npm install
npm run dev        # set VITE_API_BASE_URL=http://localhost:8999/v1 in apps/console/.env
```

Seed demo data into `apps/core/data/orion.db` with `make seed-demo-data`.

Tests and builds:

```sh
cd apps/core && go test ./...
make core-worker-build
cd apps/node && go test ./...
cd apps/console && npm run build
```

OpenAPI is generated from Core route annotations. Regenerate with `make generate-openapi` and
`make generate-sdk`; do not edit `apps/core/openapi.yaml` or the generated Console SDK by hand. Build a
Server binary with `make node-build VERSION=0.1.2`.

The public site and documentation live in [apps/website](../../apps/website/).
