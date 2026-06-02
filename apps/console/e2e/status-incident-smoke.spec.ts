import { expect, test } from "@playwright/test";
import { coreURL, signIn } from "./console-smoke-helpers";

test.describe.configure({ mode: "serial" });

test("navigates from incident list rows to incident detail", async ({ page }) => {
  await signIn(page);

  await page.goto("/incidents?agent=seed-agent-03-down");
  await expect(page.getByRole("heading", { name: "Incidents" })).toBeVisible();
  const incidentLink = page.getByRole("link", { name: "Down Server HTTP API is down" });
  await expect(incidentLink).toBeVisible();
  await incidentLink.click();

  await expect(page).toHaveURL(/\/incidents\/seed-incident-seed-monitor-seed-agent-03-down-http/);
  await expect(page.getByRole("heading", { name: "Down Server HTTP API is down" })).toBeVisible();
  await expect(page.getByRole("link", { name: "View server" })).toBeVisible();
  await expect(page.getByRole("link", { name: "View monitor" })).toBeVisible();
});

test("renders the public status page IA on desktop and mobile", async ({ page }) => {
  await page.goto("/status/seed-orion-status");
  await expect(page.getByRole("navigation", { name: "Status page navigation" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Status", exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "Events", exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "Components", exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "Get updates", exact: true })).toBeVisible();
  await expect(page.getByLabel("Current status")).toContainText(
    /systems|outage|degraded|Maintenance/i,
  );
  await expect(page.getByRole("heading", { name: "Components" })).toBeVisible();
  await expect(page.locator(".uptime-bars").first()).toBeVisible();
  await expect(page.locator(".bar").first()).toBeVisible();
  await expect(page.getByRole("heading", { name: "Get updates" })).toBeVisible();
  await expect(page.locator("body")).not.toContainText("seed-monitor");
  await expect(page.locator("body")).not.toContainText("seed-agent");

  const desktopOverflow = await page.evaluate(
    () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
  );
  expect(desktopOverflow).toBe(false);

  await page.setViewportSize({ width: 390, height: 844 });
  await page.reload();
  await expect(page.getByLabel("Current status")).toBeVisible();
  await expect(page.locator(".uptime-bars").first()).toBeVisible();
  const mobileOverflow = await page.evaluate(
    () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
  );
  expect(mobileOverflow).toBe(false);
});

test("creates a public status incident draft from an internal incident", async ({ page }) => {
  await signIn(page);

  await page.goto("/incidents/seed-incident-seed-monitor-seed-agent-03-down-http");
  await expect(page.getByRole("heading", { name: "Down Server HTTP API is down" })).toBeVisible();
  await page.getByRole("button", { name: "Public draft" }).click();

  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Seed Orion Status")).toBeVisible();
  await expect(dialog.getByText("Checkout", { exact: true })).toBeVisible();
  await expect(dialog.getByText("Investigating an issue affecting Checkout")).toBeVisible();
  await expect(dialog).not.toContainText("seed-monitor-seed-agent-03-down-http");
  await expect(dialog).not.toContainText("Down Server HTTP API is down");
  await dialog.getByRole("button", { name: "Create draft" }).click();
  await expect(dialog.getByText("Draft created:")).toBeVisible();

  const token = await page.evaluate(() => localStorage.getItem("orion_token"));
  expect(token).toBeTruthy();
  const adminIncidents = await page.request.get(
    `${coreURL}/v1/status-pages/seed-status-page-main/incidents`,
    { headers: { Authorization: `Bearer ${token}` } },
  );
  expect(adminIncidents.ok()).toBeTruthy();
  const adminPayload = (await adminIncidents.json()) as {
    data?: { incidents?: Array<{ id?: string; title?: string; visibility?: string }> };
  };
  const draftIncident = adminPayload.data?.incidents?.find(
    (incident) => incident.title === "Investigating an issue affecting Checkout",
  );
  expect(draftIncident?.visibility).toBe("draft");
  if (!draftIncident?.id) throw new Error("Draft incident was not created");

  const publicList = await page.request.get(`${coreURL}/status/seed-orion-status/incidents`);
  expect(publicList.ok()).toBeTruthy();
  expect(await publicList.text()).not.toContain("Investigating an issue affecting Checkout");
  const publicDetail = await page.request.get(
    `${coreURL}/status/seed-orion-status/incidents/${draftIncident.id}`,
  );
  expect(publicDetail.status()).toBe(404);
});

test("exercises incident detail tabs and lifecycle actions", async ({ page }) => {
  await signIn(page);

  const secretReport = await page.request.post(
    `${coreURL}/v1/agents/seed-agent-03-down/seed-monitor-seed-agent-03-down-http/report`,
    {
      headers: { Authorization: "Bearer seed-token-03-down" },
      data: {
        timestamp: new Date().toISOString(),
        health: "degraded",
        metrics: {
          status_code: 503,
          message: "checkout degraded token=console-token-value",
          authorization: "Bearer console-secret-value",
        },
      },
    },
  );
  expect(secretReport.ok()).toBeTruthy();

  await page.goto("/incidents/seed-incident-seed-monitor-seed-agent-03-down-http");
  await expect(page.getByRole("heading", { name: "Down Server HTTP API is down" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Acknowledge" })).toBeVisible();
  await expect(page.getByText("Cause / Evidence")).toBeVisible();
  await expect(page.locator("body")).not.toContainText("console-token-value");
  await expect(page.locator("body")).not.toContainText("console-secret-value");
  await page.getByRole("button", { name: "Inspect report" }).first().click();
  await expect(page.getByRole("dialog").getByText("Monitor Report")).toBeVisible();
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Inspect report" }).nth(1).click();
  await expect(page.getByRole("dialog").getByText("[redacted]").first()).toBeVisible();
  await expect(page.getByRole("dialog")).not.toContainText("console-token-value");
  await expect(page.getByRole("dialog")).not.toContainText("console-secret-value");
  await page.keyboard.press("Escape");

  await page.getByRole("tab", { name: /Notifications/ }).click();
  await expect(page.getByText("delivery failed; check Core logs")).toBeVisible();
  await page.getByRole("tab", { name: /Monitor reports/ }).click();
  await expect(page.getByText("down").first()).toBeVisible();

  await page.getByRole("button", { name: "Cover" }).click();
  await page.getByRole("dialog").getByLabel("note").fill("Known maintenance");
  await page.getByRole("dialog").getByRole("button", { name: "Cover" }).click();
  await expect(page.getByRole("button", { name: "Reopen" })).toBeVisible();
  await expect(page.getByText("Incident marked covered").first()).toBeVisible();

  await page.getByRole("button", { name: "Reopen" }).click();
  await page.getByRole("dialog").getByLabel("note").fill("Maintenance completed");
  await page.getByRole("dialog").getByRole("button", { name: "Reopen" }).click();
  await expect(page.getByRole("button", { name: "Acknowledge" })).toBeVisible();
  await expect(page.getByText("Incident reopened").first()).toBeVisible();

  await page.getByRole("button", { name: "Acknowledge" }).click();
  await page.getByRole("dialog").getByLabel("note").fill("On-call is investigating");
  await page.getByRole("dialog").getByRole("button", { name: "Acknowledge" }).click();
  await expect(page.getByRole("button", { name: "Resolve" })).toBeVisible();
  await page.getByRole("tab", { name: /Timeline/ }).click();
  await expect(page.getByText("On-call is investigating").first()).toBeVisible();

  await page.getByRole("button", { name: "Resolve" }).click();
  await page.getByRole("dialog").getByLabel("note").fill("Service recovered");
  await page.getByRole("dialog").getByRole("button", { name: "Resolve" }).click();
  await expect(page.getByRole("button", { name: "Resolve" })).toBeHidden();
  await expect(page.getByText("Incident manually resolved").first()).toBeVisible();
  await expect(page.getByText("Service recovered").first()).toBeVisible();
});
