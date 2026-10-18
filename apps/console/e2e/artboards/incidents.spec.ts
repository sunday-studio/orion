import { expect, test } from "@playwright/test";
import { expectMatchesArtboard, freezeClock, mockApi } from "./support";

test.use({ timezoneId: "UTC", viewport: { width: 1280, height: 508 } });

const shell = [
  { pattern: /\/v1\/agents/, body: { count: 0, agents: [] } },
  {
    pattern: /\/v1\/overview$/,
    body: { state: "issues", ongoing_incident_count: 2, health_issue_count: 0, stats: {} },
  },
  { pattern: /\/v1\/auth\/me$/, body: { username: "admin" } },
  { pattern: /\/v1\/health\/issues$/, body: { count: 0, issues: [] } },
  {
    pattern: /\/v1\/monitors$/,
    body: {
      count: 2,
      monitors: [
        { id: "m1", name: "checkout-https" },
        { id: "m2", name: "api-https" },
      ],
    },
  },
];

const active = [
  {
    id: "inc-231",
    number: 231,
    title: "api.example.com unreachable",
    severity: "critical",
    status: "open",
    monitor_name: "api-https",
    opened_at: "2025-10-16T14:02:12Z",
  },
  {
    id: "inc-230",
    number: 230,
    title: "db-02 disk above 90%",
    severity: "high",
    status: "acknowledged",
    monitor_name: "disk-usage",
    opened_at: "2025-10-15T22:14:00Z",
  },
];

const incidentMocks = (incidents = active, resolvedCount = 21) => [
  {
    pattern: /\/v1\/overview$/,
    body: {
      state: incidents.length ? "issues" : "ok",
      ongoing_incident_count: incidents.length,
      health_issue_count: 0,
      stats: {},
    },
  },
  ...shell.slice(1),
  {
    pattern: /\/v1\/incidents$/,
    body: {
      count: incidents.length,
      incidents,
      hidden_resolved_count: resolvedCount,
    },
  },
];

for (const theme of ["light", "dark"] as const)
  test(`Incidents list matches the artboard ${theme}`, async ({ page }) => {
    await freezeClock(page, "2025-10-16T14:32:12Z");
    await page.addInitScript((value) => localStorage.setItem("orion.theme.choice", value), theme);
    await mockApi(page, incidentMocks());
    await page.goto("/incidents");
    await page.getByText("INC-0231").waitFor();
    await page.getByRole("button", { name: "Edit Status filter" }).click();
    await page.getByText("Done", { exact: true }).waitFor();
    await expectMatchesArtboard(page, "incidents-list", 508, theme);
  });

for (const theme of ["light", "dark"] as const)
  test(`Incidents bulk selection matches the artboard ${theme}`, async ({ page }) => {
    await freezeClock(page, "2025-10-16T14:32:12Z");
    await page.addInitScript((value) => localStorage.setItem("orion.theme.choice", value), theme);
    await mockApi(page, incidentMocks());
    await page.goto("/incidents?status=open,acknowledged&since=&severity=");
    await page.getByText("INC-0231").waitFor();
    await page.getByRole("checkbox", { name: "Select all incidents" }).click();
    await expect(page.getByText("2 selected")).toBeVisible();
    await expectMatchesArtboard(page, "incidents-bulk", 460, theme);
  });

for (const theme of ["light", "dark"] as const)
  test(`Declare incident dialog matches the artboard ${theme}`, async ({ page }) => {
    await freezeClock(page, "2025-10-16T14:32:12Z");
    await page.addInitScript((value) => localStorage.setItem("orion.theme.choice", value), theme);
    await mockApi(page, incidentMocks());
    await page.goto("/incidents");
    await page.getByText("INC-0231").waitFor();
    await page.getByRole("button", { name: "Declare incident" }).click();
    await page.getByLabel("Title").fill("Checkout is slow for EU customers");
    await page.getByRole("combobox", { name: "Monitor or server" }).click();
    await page.getByRole("option", { name: "checkout-https" }).click();
    await expectMatchesArtboard(page, "incident-declare", 655, theme);
  });

for (const theme of ["light", "dark"] as const)
  test(`Incidents empty state matches the artboard ${theme}`, async ({ page }) => {
    await freezeClock(page, "2025-10-16T14:32:12Z");
    await page.addInitScript((value) => localStorage.setItem("orion.theme.choice", value), theme);
    await mockApi(page, incidentMocks([], 0));
    await page.goto("/incidents");
    await page.getByText("No open incidents").waitFor();
    await expectMatchesArtboard(page, "incidents-empty", 460, theme);
  });

test("Bulk action and declare send the Core payloads", async ({ page }) => {
  await freezeClock(page, "2025-10-16T14:32:12Z");
  await mockApi(page, incidentMocks());
  const bodies: Record<string, unknown> = {};
  await page.route(/\/v1\/incidents\/bulk$/, async (route) => {
    bodies.bulk = route.request().postDataJSON();
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ success: true, data: { results: [], succeeded: 2, failed: 0 } }),
    });
  });
  await page.route(/\/v1\/incidents$/, async (route) => {
    if (route.request().method() !== "POST") return route.fallback();
    bodies.declare = route.request().postDataJSON();
    return route.fulfill({
      contentType: "application/json",
      status: 409,
      body: JSON.stringify({ success: false, error: "active incident exists" }),
    });
  });
  await page.goto("/incidents?status=open,acknowledged&since=&severity=");
  await page.getByText("INC-0231").waitFor();
  await page.getByRole("checkbox", { name: "Select all incidents" }).click();
  await page.getByRole("button", { name: "Resolve", exact: true }).click();
  await expect
    .poll(() => bodies.bulk)
    .toMatchObject({
      action: "resolve",
      ids: ["inc-231", "inc-230"],
    });

  await page.getByRole("button", { name: "Declare incident" }).click();
  await page.getByLabel("Title").fill("Checkout is slow");
  await page.getByRole("combobox", { name: "Monitor or server" }).click();
  await page.getByRole("option", { name: "checkout-https" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Declare incident" }).click();
  await expect(page.getByText("already has an active incident")).toBeVisible();
  expect(bodies.declare).toMatchObject({
    title: "Checkout is slow",
    severity: "high",
    monitor_id: "m1",
    notify: true,
  });
});
