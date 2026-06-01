import { defineConfig, devices } from "@playwright/test";

const consolePort = process.env.ORION_E2E_CONSOLE_PORT ?? "5173";
const consoleURL = `http://127.0.0.1:${consolePort}`;

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  reporter: "list",
  use: {
    baseURL: consoleURL,
    trace: "on-first-retry",
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
