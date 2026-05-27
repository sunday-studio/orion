# M6: Core Monitor Catalog Expansion

## Goal

Make Core-managed monitors cover the first broad service-check catalog from `docs/plans/core-managed-monitors.md`, with each runner producing bounded report payloads that feed the existing Core monitor scheduler and report service.

## Scope

This milestone covers the Core worker monitor runner catalog, not the Console creation workflow or public status page mapping. The worker now dispatches these Core monitor kinds:

- `http`, `http_status`, `http_keyword`, and `expected_status`;
- `tcp` and `tcp_port`;
- `dns`;
- `tls` and `tls_certificate`;
- `udp`;
- `api_request`;
- `domain_expiration`;
- `ping`;
- `mail`, `smtp`, `imap`, `pop`, and `pop3`;
- `synthetic` and `synthetic_multi_step`;
- `playwright` and `playwright_transaction`.

## Completed

- Split the Core worker loop into focused runner files so each monitor type owns its config parsing, execution result, report payload, and tests.
- Expanded HTTP checks with expected status sets plus required and forbidden response body keywords, including dedicated dispatch aliases for keyword and expected-status monitor kinds.
- Added TCP, DNS, TLS certificate, UDP, API request, domain expiration, ping-style reachability, mail protocol, synthetic multi-step, and Playwright transaction runners.
- Kept runner behavior bounded: response samples are capped, Playwright artifacts are capped, synthetic variables are limited, UDP requires an expected response, and browser execution is behind an explicit runtime boundary.
- Preserved safe secret handling: API request secret headers and Playwright secret variables are applied through secret config but only redacted key names are reported.
- Documented first-release behavior and known fallbacks in `docs/plans/core-managed-monitors.md`.

## Evidence

- `0768116 feat(core): run HTTP status monitors`
- `84ec43d feat(core): expand HTTP monitor checks`
- `3127449 feat(core): add TCP monitor runner`
- `2e75d44 refactor(core): split worker monitor runners`
- `4dc090b feat(core): add DNS monitor runner`
- `c13687f feat(core): add TLS certificate monitor`
- `8fc02d4 feat(core): add UDP monitor runner`
- `3af4813 feat(core): add API request monitor`
- `3594f6c feat(core): add domain expiration monitor`
- `d43b00f feat(core): add ping monitor runner`
- `2546463 feat(core): add mail protocol monitors`
- `b3af9b7 feat(core): add synthetic monitor runner`
- `59dfded feat(core): add Playwright transaction runner`

## Verification

- `GOCACHE=/private/tmp/orion-go-cache go test ./internal/worker` in `apps/core`.
- `GOCACHE=/private/tmp/orion-go-cache go test ./...` in `apps/core`.
- `maat validate --storage /Users/casprine/Desktop/vendor/personal/maat-storage`.

## Open Risks

- ICMP ping is explicit unsupported/permission behavior until the worker has privileged raw-socket support.
- Domain expiration uses RDAP first and reports unavailable data clearly; WHOIS fallback remains deferred.
- Playwright transactions require an explicit `ORION_PLAYWRIGHT_RUNNER` runtime on the worker host; the default Core image does not silently bundle browsers.
- Mail monitors intentionally do not authenticate in this milestone.

## Next

- Wire the full catalog into Console creation and edit flows.
- Add Core monitor type-specific API validation if the admin API still accepts only generic config blobs.
- Decide packaging and deployment for an optional Playwright runtime image or sidecar.
