# Orion Console

React and Vite app for the Orion web console.

## Commands

```sh
pnpm run dev
pnpm run build
pnpm run lint
pnpm run format:check
pnpm run format
pnpm run test:e2e
```

Linting and formatting use OXC:

- `oxlint` for linting;
- `oxfmt` for formatting.

Do not add Biome or ESLint config to this app.

## E2E Smoke

The Playwright smoke suite starts a temporary Orion Core, seeds demo data into a temporary SQLite
database, generates the ignored Console API client, then starts Vite against that Core.

Prerequisites:

- Go toolchain with Core module dependencies available, or network access for the first run.
- Console dependencies installed with `pnpm install`.
- Playwright browser binaries installed for `@playwright/test`.

Default local ports:

- Console: `45173`.
- Core: `48999`.

Override them when needed:

```sh
ORION_CONSOLE_E2E_PORT=45174 ORION_CORE_E2E_PORT=49000 pnpm run test:e2e
```
