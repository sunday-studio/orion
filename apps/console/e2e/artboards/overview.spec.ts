import { test } from "@playwright/test";
import { expectMatchesArtboard, freezeClock, mockApi } from "./support";

test.use({ timezoneId: "UTC", viewport: { width: 1280, height: 779 } });

const needsYou = {
  id: "inc-1",
  title: "api.example.com is down",
  severity: "critical",
  status: "open",
  opened_at: "2025-10-16T14:02:00Z",
  notified_at: "2025-10-16T14:02:00Z",
  summary: "check failed, timeout after 10s.",
};

const ongoing = [
  needsYou,
  {
    id: "inc-2",
    title: "db-02 disk above 90%",
    severity: "medium",
    status: "open",
    opened_at: "2025-10-15T22:14:00Z",
  },
];

const recent = [
  {
    id: "r1",
    title: "web-https latency",
    status: "resolved",
    opened_at: "2025-10-13T09:10:00Z",
    duration_seconds: 1320,
  },
  {
    id: "r2",
    title: "edge-02 missed reports",
    status: "resolved",
    opened_at: "2025-10-12T03:44:00Z",
    duration_seconds: 480,
  },
  {
    id: "r3",
    title: "nightly-backup heartbeat missed",
    status: "resolved",
    opened_at: "2025-10-11T02:05:00Z",
    duration_seconds: 4320,
  },
];

const issues = {
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
};

const baseStats = { monitors_total: 24, monitors_up: 21, servers_total: 6, servers_reporting: 5 };

const mainMocks = [
  {
    pattern: /\/v1\/overview$/,
    body: {
      state: "issues",
      needs_you: needsYou,
      ongoing_incident_count: 2,
      ongoing_incidents: ongoing,
      recent_incidents: recent,
      health_issue_count: 3,
      stats: baseStats,
    },
  },
  { pattern: /\/v1\/auth\/me$/, body: { username: "admin", auth_enabled: true } },
  { pattern: /\/v1\/health\/issues$/, body: issues },
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
      pattern: /\/v1\/overview$/,
      body: {
        state: "ok",
        ongoing_incident_count: 0,
        ongoing_incidents: [],
        recent_incidents: recent,
        health_issue_count: 0,
        stats: { monitors_total: 24, monitors_up: 24, servers_total: 6, servers_reporting: 6 },
      },
    },
    { pattern: /\/v1\/auth\/me$/, body: { username: "admin", auth_enabled: true } },
    { pattern: /\/v1\/health\/issues$/, body: { count: 0, issues: [] } },
  ]);
  await page.goto("/");
  await page.getByText(/Checked \d+ seconds? ago/).waitFor();
  await expectMatchesArtboard(page, "overview-all-good", 640);
});

test("Overview first run matches the onboarding artboard", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:32:12Z");
  await mockApi(page, [
    {
      pattern: /\/v1\/overview$/,
      body: { state: "empty", ongoing_incident_count: 0, health_issue_count: 0, stats: {} },
    },
    { pattern: /\/v1\/auth\/me$/, body: { username: "admin", auth_enabled: true } },
    { pattern: /\/v1\/health\/issues$/, body: { count: 0, issues: [] } },
  ]);
  await page.goto("/");
  await page.getByText("Nothing is monitored yet").waitFor();
  await expectMatchesArtboard(page, "onboarding", 460);
});
