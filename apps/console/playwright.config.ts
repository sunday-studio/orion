import { defineConfig, devices } from "@playwright/test";

const consolePort = process.env.ORION_E2E_CONSOLE_PORT ?? "5173";
const consoleURL = `http://127.0.0.1:${consolePort}`;

export default defineConfig({
  // Artboard comparisons read the exported canvas PNGs straight from docs/design.
  snapshotPathTemplate: "{testDir}/../../../docs/design/artboards/png/{arg}{ext}",
  expect: { toHaveScreenshot: { threshold: 0.2 } },
  testDir: "./e2e",
  fullyParallel: false,
  reporter: "list",
  workers: 1,
  // Core can briefly return "database is locked" under seed-time writes; retry in CI only.
  retries: process.env.CI ? 2 : 0,
  use: {
    baseURL: consoleURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  webServer: {
    command: "node e2e/support/start-core-smoke.mjs",
    url: consoleURL,
    reuseExistingServer: false,
    timeout: 120_000,
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
