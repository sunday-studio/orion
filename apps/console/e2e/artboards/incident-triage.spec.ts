import { expect, test } from "@playwright/test";
import { expectMatchesArtboard, freezeClock, mockApi } from "./support";

test.use({ timezoneId: "UTC", viewport: { width: 1280, height: 460 } });

const shell = [
  {
    pattern: /\/v1\/overview$/,
    body: { state: "issues", ongoing_incident_count: 2, health_issue_count: 0, stats: {} },
  },
  { pattern: /\/v1\/auth\/me$/, body: { username: "admin" } },
];

for (const theme of ["light", "dark"] as const)
  test(`Triage matches the artboard ${theme}`, async ({ page }) => {
    await freezeClock(page, "2025-10-16T14:32:12Z");
    await page.addInitScript((value) => localStorage.setItem("orion.theme.choice", value), theme);
    await mockApi(page, [
      ...shell,
      {
        pattern: /\/v1\/incidents\/candidates$/,
        body: {
          count: 3,
          candidates: [
            {
              monitor_id: "m1",
              monitor_name: "checkout-https",
              agent_name: "edge-01",
              monitor_type: "http",
              health: "degraded",
              severity: "medium",
              detail: "p95 over 1.5s for 10 minutes",
              seen_at: "2025-10-16T14:20:00Z",
            },
            {
              monitor_id: "m2",
              monitor_name: "disk-usage",
              agent_name: "edge-03",
              monitor_type: "agent",
              health: "stale",
              severity: "high",
              detail: "No report for 12 minutes",
              seen_at: "2025-10-16T14:18:00Z",
            },
            {
              monitor_id: "m3",
              monitor_name: "nightly-backup",
              agent_name: "edge-01",
              monitor_type: "heartbeat",
              health: "down",
              severity: "high",
              detail: "3 of 5 reports down",
              seen_at: "2025-10-16T14:00:00Z",
            },
          ],
        },
      },
    ]);
    await page.goto("/incidents/triage");
    await page.getByText("3 in the inbox").waitFor();
    await expectMatchesArtboard(page, "incident-candidates", 460, theme);
  });

test("Triage opens an incident from a candidate", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:32:12Z");
  await mockApi(page, [
    ...shell,
    {
      pattern: /\/v1\/incidents\/candidates$/,
      body: {
        count: 1,
        candidates: [
          { monitor_id: "m1", monitor_name: "checkout-https", health: "down", severity: "high" },
        ],
      },
    },
  ]);
  let opened = false;
  await page.route(/\/v1\/incidents\/candidates\/m1\/open$/, async (route) => {
    opened = true;
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ success: true, data: { incident: { id: "inc-9" } } }),
    });
  });
  await page.goto("/incidents/triage");
  await page.getByRole("button", { name: "Open incident" }).click();
  await expect.poll(() => opened).toBe(true);
  await expect(page).toHaveURL(/\/incidents\/inc-9$/);
});

for (const theme of ["light", "dark"] as const)
  test(`Health issues matches the artboard ${theme}`, async ({ page }) => {
    await freezeClock(page, "2025-10-16T14:32:12Z");
    await page.addInitScript((value) => localStorage.setItem("orion.theme.choice", value), theme);
    await mockApi(page, [
      ...shell,
      {
        pattern: /\/v1\/health\/issues$/,
        body: {
          count: 4,
          issues: [
            {
              kind: "monitor",
              target_id: "m1",
              target_name: "api-https",
              agent_name: "edge-01",
              health: "down",
              detail: "timeout after 10s",
              since: "2025-10-16T14:02:00Z",
              incident_id: "inc-231",
              incident_display_id: "INC-0231",
            },
            {
              kind: "monitor",
              target_id: "m2",
              target_name: "checkout-https",
              agent_name: "edge-01",
              health: "degraded",
              detail: "p95 1.8s, limit 1.5s",
              since: "2025-10-16T14:20:00Z",
            },
            {
              kind: "server",
              target_id: "a1",
              target_name: "db-02",
              agent_name: "db-02",
              health: "degraded",
              detail: "disk at 91%",
              since: "2025-10-16T12:32:00Z",
            },
            {
              kind: "monitor",
              target_id: "m4",
              target_name: "disk-usage",
              agent_name: "edge-03",
              health: "degraded",
              issue_type: "stale_data",
              detail: "No recent data",
              since: "2025-10-16T14:20:00Z",
            },
          ],
        },
      },
    ]);
    await page.goto("/health-issues");
    await page.getByText("INC-0231").waitFor();
    await expectMatchesArtboard(page, "health-issues", 460, theme);
  });
