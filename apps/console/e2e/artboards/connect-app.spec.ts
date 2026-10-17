import { test } from "@playwright/test";
import { expectMatchesArtboard, freezeClock, mockApi } from "./support";

test.use({ timezoneId: "UTC", viewport: { width: 1280, height: 477 } });

const baseMocks = [
  {
    pattern: /\/v1\/health\/summary$/,
    body: { overall_health: "up", monitors: { total: 24, up: 24 }, agents: { total: 6 } },
  },
  { pattern: /\/v1\/health\/issues$/, body: { count: 0, issues: [] } },
  { pattern: /\/v1\/incidents$/, body: { count: 0, incidents: [] } },
  {
    pattern: /\/v1\/apps\/app-1\/keys$/,
    body: { key: { id: "k1", key: "orn_app_7Qx2m9a1c4e8b05d3f6a2c9e1b7d4f80" } },
  },
  { pattern: /\/v1\/apps$/, body: { app: { id: "app-1", name: "billing-api" }, apps: [] } },
];

const startFlow = async (page: import("@playwright/test").Page, appMock: object) => {
  await freezeClock(page, "2025-10-16T14:35:00Z");
  await mockApi(page, [{ pattern: /\/v1\/apps\/app-1$/, body: appMock }, ...baseMocks]);
  await page.goto("/apps/new");
};

test("Connect app: create matches the artboard", async ({ page }) => {
  await startFlow(page, { app: { id: "app-1", name: "billing-api" } });
  await page.getByLabel("Name").fill("billing-api");
  await expectMatchesArtboard(page, "app-connect-create", 477);
});

test("Connect app: add the key matches the artboard", async ({ page }) => {
  await startFlow(page, { app: { id: "app-1", name: "billing-api" } });
  await page.getByLabel("Name").fill("billing-api");
  await page.getByRole("button", { name: "Create app" }).click();
  await page.getByText("Shown once").waitFor();
  await expectMatchesArtboard(page, "app-connect-snippet", 629);
});

test("Connect app: waiting matches the artboard", async ({ page }) => {
  await startFlow(page, { app: { id: "app-1", name: "billing-api" } });
  await page.getByLabel("Name").fill("billing-api");
  await page.getByRole("button", { name: "Create app" }).click();
  await page.getByRole("button", { name: "I sent one" }).click();
  await page.getByText("No log received yet").waitFor();
  await expectMatchesArtboard(page, "app-connect-waiting", 460);
});

test("Connect app: received matches the artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:35:05Z");
  let ingested = false;
  await mockApi(page, [
    {
      pattern: /\/v1\/apps\/app-1$/,
      body: () => ({
        app: {
          id: "app-1",
          name: "billing-api",
          last_ingest_at: ingested ? "2025-10-16T14:35:02Z" : undefined,
        },
      }),
    },
    {
      pattern: /\/v1\/logs$/,
      body: {
        entries: [
          {
            id: "l1",
            timestamp: "2025-10-16T14:35:02.118Z",
            severity: "info",
            body: "Hello from Orion",
          },
        ],
      },
    },
    ...baseMocks,
  ]);
  await page.goto("/apps/new");
  await page.getByLabel("Name").fill("billing-api");
  await page.getByRole("button", { name: "Create app" }).click();
  await page.getByRole("button", { name: "I sent one" }).click();
  await page.getByText("No log received yet").waitFor();
  ingested = true;
  await page.clock.fastForward(3000);
  await page.getByText("Its first log just arrived.").waitFor();
  await expectMatchesArtboard(page, "app-connect-received", 460);
});
