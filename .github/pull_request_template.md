## Summary

-

## Validation

- [ ] Core backend: `cd apps/core && go test ./...`
- [ ] Core coverage: `make core-coverage`
- [ ] Agent: `cd apps/agent && go test ./...`
- [ ] Console: `cd apps/console && npm run build`
- [ ] Generated contracts: `make generate-sdk` and committed generated output
- [ ] Not applicable because:

## Core Backend Coverage

- [ ] This PR does not change Core backend behavior.
- [ ] New or changed Core routes include API/integration coverage.
- [ ] New or changed Core services include focused service coverage.
- [ ] New or changed Core worker behavior includes success, failure, timeout, and redaction coverage where relevant.
- [ ] New or changed Core migrations include migration or compatibility coverage when defaults, indexes, or existing rows are affected.
- [ ] No Core backend test changes because:
