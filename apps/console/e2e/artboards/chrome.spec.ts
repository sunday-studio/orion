import { test } from "@playwright/test";
import { expectMatchesArtboard, freezeClock, mockApi } from "./support";

test.use({ timezoneId: "UTC", viewport: { width: 1280, height: 640 } });

const healthMocks = [
  {
    pattern: /\/v1\/health\/summary$/,
    body: { overall_health: "degraded", monitors: { total: 24, up: 21 }, agents: { total: 6 } },
  },
  { pattern: /\/v1\/agents\/summary$/, body: { total: 6, up: 5 } },
  { pattern: /\/v1\/health\/issues$/, body: { count: 0, issues: [] } },
  {
    pattern: /\/v1\/incidents$/,
    body: (url: URL) => ({
      count: 2,
      incidents: url.searchParams.get("status")
        ? [
            {
              id: "i1",
              title: "api.example.com unreachable",
              severity: "high",
              status: "open",
              opened_at: "2025-10-16T14:02:00Z",
            },
          ]
        : [
            {
              id: "i1",
              title: "api.example.com unreachable",
              severity: "high",
              status: "open",
              opened_at: "2025-10-16T14:02:00Z",
            },
          ],
    }),
  },
  {
    pattern: /\/v1\/monitors$/,
    body: {
      count: 2,
      monitors: [
        { id: "m1", name: "api-https", health: "down" },
        { id: "m2", name: "api-tcp-5432", health: "up" },
      ],
    },
  },
];

test("Command palette matches the artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:32:12Z");
  await mockApi(page, healthMocks);
  await page.goto("/");
  await page.getByRole("button", { name: "Search" }).click();
  await page.getByLabel("Search destinations").fill("api");
  await page.getByText("api-tcp-5432").waitFor();
  await expectMatchesArtboard(page, "command-palette", 640);
});

test("Shortcuts sheet matches the artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:32:12Z");
  await mockApi(page, healthMocks);
  await page.goto("/");
  await page.keyboard.press("?");
  await page.getByText("Keyboard shortcuts").waitFor();
  await expectMatchesArtboard(page, "shortcuts", 640);
});

test("Account menu matches the artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:32:12Z");
  await mockApi(page, healthMocks);
  await page.goto("/");
  await page.getByRole("button", { name: "Account menu" }).click();
  await page.getByText("Signed in as admin").waitFor();
  await expectMatchesArtboard(page, "avatar-menu", 460);
});
