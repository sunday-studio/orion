import { defineConfig, devices } from "@playwright/test";

const consolePort = Number(process.env.ORION_CONSOLE_E2E_PORT ?? 45173);
const corePort = Number(process.env.ORION_CORE_E2E_PORT ?? 48999);
const baseURL = `http://127.0.0.1:${consolePort}`;

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  reporter: "list",
  use: {
    baseURL,
    trace: "on-first-retry",
  },
  webServer: {
    command: `node e2e/support/start-core-smoke.mjs --console-port ${consolePort} --core-port ${corePort}`,
    url: baseURL,
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
