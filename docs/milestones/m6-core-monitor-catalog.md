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
- `synthetic` and `synthetic_multi_step`.

## Completed

- Split the Core worker loop into focused runner files so each monitor type owns its config parsing, execution result, report payload, and tests.
- Expanded HTTP checks with expected status sets plus required and forbidden response body keywords, including dedicated dispatch aliases for keyword and expected-status monitor kinds.
- Added TCP, DNS, TLS certificate, UDP, API request, domain expiration, ping-style reachability, mail protocol, and synthetic multi-step runners.
- Kept runner behavior bounded: response samples are capped, synthetic variables are limited, and UDP requires an expected response.
- Preserved safe secret handling for API request secret headers.
- Documented first-release behavior and known fallbacks in `docs/plans/core-managed-monitors.md`.
- Removed browser transaction monitors from first-release scope; legacy configs now report an unsupported monitor result.

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

## Verification

- `GOCACHE=/private/tmp/orion-go-cache go test ./internal/worker` in `apps/core`.
- `GOCACHE=/private/tmp/orion-go-cache go test ./...` in `apps/core`.
- `maat validate --storage /Users/casprine/Desktop/vendor/personal/maat-storage`.

## Open Risks

- ICMP ping is explicit unsupported/permission behavior until the worker has privileged raw-socket support.
- Domain expiration uses RDAP first and reports unavailable data clearly; WHOIS fallback remains deferred.
- Browser transaction checks remain outside the first-release worker runtime until sandboxing and artifact retention have a versioned contract.
- Mail monitors intentionally do not authenticate in this milestone.

## Next

- Wire the full catalog into Console creation and edit flows.
