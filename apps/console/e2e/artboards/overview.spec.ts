import { test } from "@playwright/test";
import { expectMatchesArtboard, freezeClock, mockApi } from "./support";

test.use({ timezoneId: "UTC", viewport: { width: 1280, height: 779 } });

const incidents = {
  incidents: [
    {
      id: "inc-1",
      title: "api.example.com is down",
      severity: "critical",
      status: "open",
      opened_at: "2025-10-16T14:02:00Z",
      last_event_at: "2025-10-16T14:02:00Z",
      notification_status: "sent",
      latest_event: "check failed, timeout after 10s.",
      allowed_actions: { acknowledge: { allowed: true } },
    },
    {
      id: "inc-2",
      title: "db-02 disk above 90%",
      severity: "medium",
      status: "open",
      opened_at: "2025-10-15T22:14:00Z",
    },
  ],
  count: 2,
};

const mainMocks = [
  {
    pattern: /\/v1\/health\/summary$/,
    body: { overall_health: "degraded", monitors: { total: 24, up: 21 }, agents: { total: 6 } },
  },
  { pattern: /\/v1\/agents\/summary$/, body: { total: 6, up: 5 } },
  {
    pattern: /\/v1\/health\/issues$/,
    body: {
      count: 3,
      issues: [
        {
          monitor_id: "m1",
          monitor_name: "api-https",
          agent_name: "edge-01",
          monitor_type: "http",
          health: "down",
        },
        {
          monitor_id: "m2",
          monitor_name: "checkout-https",
          agent_name: "edge-01",
          monitor_type: "http",
          health: "degraded",
        },
        {
          monitor_id: "m3",
          monitor_name: "edge-03 disk-usage",
          agent_name: "edge-03",
          health: "stale",
          issue_type: "stale_data",
        },
      ],
    },
  },
  {
    pattern: /\/v1\/incidents$/,
    body: (url: URL) =>
      url.searchParams.get("status") === "resolved"
        ? {
            count: 3,
            incidents: [
              {
                id: "r1",
                title: "web-https latency",
                status: "resolved",
                opened_at: "2025-10-13T09:10:00Z",
                resolved_at: "2025-10-13T09:32:00Z",
              },
              {
                id: "r2",
                title: "edge-02 missed reports",
                status: "resolved",
                opened_at: "2025-10-12T03:44:00Z",
                resolved_at: "2025-10-12T03:52:00Z",
              },
              {
                id: "r3",
                title: "nightly-backup heartbeat missed",
                status: "resolved",
                opened_at: "2025-10-11T02:05:00Z",
                resolved_at: "2025-10-11T03:17:00Z",
              },
            ],
          }
        : incidents,
  },
];

test("Overview matches the Main artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:32:12Z");
  await mockApi(page, mainMocks);
  await page.goto("/");
  await page.getByText("Needs attention").waitFor();
  await expectMatchesArtboard(page, "Main", 779);
});

test("Overview dark matches the dashboard-dark artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:32:12Z");
  await mockApi(page, mainMocks, { theme: "dark" });
  await page.goto("/");
  await page.getByText("Needs attention").waitFor();
  await expectMatchesArtboard(page, "dashboard-dark", 783, "dark");
});

test("Overview all good matches the artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:32:12Z");
  await mockApi(page, [
    {
      pattern: /\/v1\/health\/summary$/,
      body: { overall_health: "up", monitors: { total: 24, up: 24 }, agents: { total: 6 } },
    },
    { pattern: /\/v1\/agents\/summary$/, body: { total: 6, up: 6 } },
    { pattern: /\/v1\/health\/issues$/, body: { count: 0, issues: [] } },
    {
      pattern: /\/v1\/incidents$/,
      body: (url: URL) =>
        url.searchParams.get("status") === "resolved"
          ? {
              count: 3,
              incidents: [
                {
                  id: "r1",
                  title: "web-https latency",
                  status: "resolved",
                  opened_at: "2025-10-13T09:10:00Z",
                  resolved_at: "2025-10-13T09:32:00Z",
                },
                {
                  id: "r2",
                  title: "edge-02 missed reports",
                  status: "resolved",
                  opened_at: "2025-10-12T03:44:00Z",
                  resolved_at: "2025-10-12T03:52:00Z",
                },
                {
                  id: "r3",
                  title: "nightly-backup heartbeat missed",
                  status: "resolved",
                  opened_at: "2025-10-11T02:05:00Z",
                  resolved_at: "2025-10-11T03:17:00Z",
                },
              ],
            }
          : { count: 0, incidents: [] },
    },
  ]);
  await page.goto("/");
  await page.getByText(/Checked \d+ seconds? ago/).waitFor();
  await expectMatchesArtboard(page, "overview-all-good", 640);
});

test("Overview first run matches the onboarding artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:32:12Z");
  await mockApi(page, [
    {
      pattern: /\/v1\/health\/summary$/,
      body: { overall_health: "unknown", monitors: { total: 0 }, agents: { total: 0 } },
    },
    { pattern: /\/v1\/agents\/summary$/, body: { total: 0, up: 0 } },
    { pattern: /\/v1\/health\/issues$/, body: { count: 0, issues: [] } },
    { pattern: /\/v1\/incidents$/, body: { count: 0, incidents: [] } },
  ]);
  await page.goto("/");
  await page.getByText("Nothing is monitored yet").waitFor();
  await expectMatchesArtboard(page, "onboarding", 460);
});
