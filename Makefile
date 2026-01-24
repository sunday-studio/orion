.PHONY: generate-sdk
generate-sdk:
	npx openapi-typescript core/openapi.yaml -o sdk/api.d.ts
