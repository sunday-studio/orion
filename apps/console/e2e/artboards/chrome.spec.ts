import { test } from "@playwright/test";
import { expectMatchesArtboard, freezeClock, mockApi } from "./support";

test.use({ timezoneId: "UTC", viewport: { width: 1280, height: 640 } });

const incident = {
  id: "i1",
  title: "api.example.com is down",
  severity: "critical",
  status: "open",
  opened_at: "2025-10-16T14:02:00Z",
  notified_at: "2025-10-16T14:02:00Z",
  summary: "check failed, timeout after 10s.",
};

const healthMocks = [
  {
    pattern: /\/v1\/overview$/,
    body: {
      state: "issues",
      needs_you: incident,
      ongoing_incident_count: 2,
      ongoing_incidents: [incident],
      recent_incidents: [],
      health_issue_count: 0,
      stats: { monitors_total: 24, monitors_up: 21, servers_total: 6, servers_reporting: 5 },
    },
  },
  { pattern: /\/v1\/auth\/me$/, body: { username: "admin", auth_enabled: true } },
  { pattern: /\/v1\/health\/issues$/, body: { count: 0, issues: [] } },
  {
    pattern: /\/v1\/search$/,
    body: {
      count: 3,
      hits: [
        { type: "monitor", id: "m1", name: "api-https", status: "down" },
        { type: "monitor", id: "m2", name: "api-tcp-5432", status: "up" },
        { type: "incident", id: "i1", name: "api.example.com unreachable", status: "open" },
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
