import { type Page, expect, test } from "@playwright/test";

const username = "admin";
const password = "change-me";
const coreURL = `http://127.0.0.1:${process.env.ORION_CORE_E2E_PORT ?? 48999}`;

test.describe.configure({ mode: "serial" });

const signIn = async (page: Page) => {
  await page.goto("/login");
  await page.getByPlaceholder("Username").fill(username);
  await page.getByPlaceholder("Password").fill(password);
  await page.getByRole("button", { name: "Enter" }).click();
  await expect(page.getByRole("heading", { name: "Incidents" })).toBeVisible();
};

const primaryNavLink = (page: Page, name: string) =>
  page.getByLabel("Primary navigation").getByRole("link", { name });

test("signs in, rejects bad credentials, and signs out", async ({ page }) => {
  await page.goto("/login");
  await page.getByPlaceholder("Username").fill(username);
  await page.getByPlaceholder("Password").fill("wrong-password");
  await page.getByRole("button", { name: "Enter" }).click();
  await expect(page.getByText("Invalid credentials")).toBeVisible();

  await page.getByPlaceholder("Password").fill(password);
  await page.getByRole("button", { name: "Enter" }).click();
  await expect(page.getByRole("heading", { name: "Incidents" })).toBeVisible();

  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page.getByPlaceholder("Username")).toBeVisible();
});

test("renders primary operations pages with seeded Core data", async ({ page }) => {
  await signIn(page);

  await page.goto("/agents");
  await expect(page).toHaveURL(/\/servers$/);
  await expect(page.getByRole("heading", { name: "Servers" })).toBeVisible();

  await page.goto("/agents/seed-agent-01-healthy");
  await expect(page).toHaveURL(/\/servers\/seed-agent-01-healthy$/);
  await expect(page.getByRole("heading", { name: "Healthy Server" })).toBeVisible();

  await primaryNavLink(page, "Servers").click();
  await expect(page.getByRole("heading", { name: "Servers" })).toBeVisible();
  await expect(page.getByText("Healthy Server", { exact: true })).toBeVisible();
  await expect(page.getByText("9 monitors").first()).toBeVisible();

  await primaryNavLink(page, "Monitors").click();
  await expect(page.getByRole("heading", { name: "Monitors" })).toBeVisible();
  await page.getByPlaceholder("Search monitors").fill("Healthy Server HTTP API");
  await expect(page.getByText("Healthy Server HTTP API")).toBeVisible();

  await page.goto("/monitors?owner=core&type=http&source=core");
  await expect(page.getByRole("heading", { name: "Monitors" })).toBeVisible();
  await expect(page.getByText("Core Public API")).toBeVisible();
  await expect(page.getByText("Core", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Orion Core")).toBeVisible();
  await expect(page.getByText("Owner: Core")).toBeVisible();
  await expect(page.getByText("Type: HTTP")).toBeVisible();
  await expect(page.getByText("Source: Core")).toBeVisible();

  await primaryNavLink(page, "Alerts").click();
  await expect(page.getByRole("heading", { name: "Alerts" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "Notification Log" })).toBeVisible();
  await page.getByRole("tab", { name: "Rules" }).click();
  await expect(page.getByRole("heading", { name: "Rules" })).toBeVisible();

  await primaryNavLink(page, "Status").click();
  await expect(page.getByRole("heading", { name: "Status Pages" })).toBeVisible();

  await primaryNavLink(page, "Logs").click();
  await expect(page.getByRole("heading", { name: "Logs" })).toBeVisible();
  await expect(page.getByPlaceholder("Search events")).toBeVisible();

  await primaryNavLink(page, "Settings").click();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  await expect(page.getByLabel("Raw report days")).toHaveValue("30");
});

test("creates and manages a Core HTTP monitor", async ({ page }) => {
  const monitorName = `Core E2E HTTP ${Date.now()}`;
  const updatedName = `${monitorName} updated`;

  await signIn(page);

  await primaryNavLink(page, "Monitors").click();
  await page.getByRole("button", { name: "Core monitor" }).click();
  await page.getByLabel("Name").fill(monitorName);
  await page.getByLabel("URL").fill(`${coreURL}/health`);
  await page.getByLabel("Expected status").fill("503");
  await page.getByLabel("Interval seconds").fill("45");
  await page.getByRole("button", { name: "Create and test" }).click();
  await expect(page.getByText("received HTTP 200, expected 503")).toBeVisible();

  await page.getByPlaceholder("Search monitors").fill(monitorName);
  await expect(page.getByRole("link", { name: monitorName })).toBeVisible();
  await page.getByRole("link", { name: monitorName }).click();
  await expect(page.getByRole("heading", { name: monitorName })).toBeVisible();
  await expect(page.getByText("Core · http")).toBeVisible();

  await page.getByRole("tab", { name: "Configuration" }).click();
  await expect(page.getByText(`${coreURL}/health`)).toBeVisible();
  await expect(page.getByLabel("Configuration").getByText("45s")).toBeVisible();

  await page.getByRole("button", { name: "Test" }).click();
  await expect(page.getByText("Core monitor test reported down.")).toBeVisible();

  await page.getByRole("button", { name: "Pause" }).click();
  await expect(page.getByText("Core monitor paused.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Resume" })).toBeVisible();

  await page.getByRole("button", { name: "Resume" }).click();
  await expect(page.getByText("Core monitor resumed.")).toBeVisible();

  await page.getByRole("button", { name: "Edit" }).click();
  await page.getByLabel("Name").fill(updatedName);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("heading", { name: updatedName })).toBeVisible();

  await page.getByRole("button", { name: "Delete" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByRole("heading", { name: "Monitors" })).toBeVisible();
});

test("creates a Core heartbeat monitor and shows setup affordances", async ({ page }) => {
  const monitorName = `Core E2E Heartbeat ${Date.now()}`;

  await signIn(page);

  await primaryNavLink(page, "Monitors").click();
  await page.getByRole("button", { name: "Core monitor" }).click();
  await page.getByLabel("Name").fill(monitorName);
  await page.getByLabel("Core monitor type").selectOption("heartbeat");
  await expect(page.getByLabel("URL")).toBeHidden();
  await page.getByLabel("Interval seconds").fill("90");
  await page.getByLabel("Grace seconds").fill("30");
  await page.getByRole("button", { name: "Create" }).click();

  await expect(page.getByText("Heartbeat monitor created.")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Heartbeat Setup" })).toBeVisible();
  await expect(page.getByText("Waiting for the first signal.")).toBeVisible();
  await expect(page.getByText("/v1/heartbeats/").first()).toBeVisible();
  await expect(page.getByText("/success").first()).toBeVisible();
  await expect(page.getByText("/failure").first()).toBeVisible();
  await expect(page.getByText("curl -fsS -X POST").first()).toBeVisible();
  await expect(page.getByRole("button", { name: "Copy" }).first()).toBeVisible();

  const endpointFor = async (suffix: string) => {
    const endpoint = await page
      .locator("pre")
      .filter({ hasText: "/v1/heartbeats/" })
      .filter({ hasText: suffix })
      .first()
      .textContent();
    expect(endpoint).toBeTruthy();
    return endpoint?.trim() ?? "";
  };
  const successEndpoint = await endpointFor("/success");
  const failureEndpoint = await endpointFor("/failure");

  const failureResponse = await page.request.post(failureEndpoint, {
    data: "password=super-secret token=raw-token-value",
  });
  expect(failureResponse.ok()).toBeTruthy();
  const successResponse = await page.request.post(successEndpoint, { data: "status=ok" });
  expect(successResponse.ok()).toBeTruthy();

  await page.getByRole("link", { name: "Open monitor" }).click();
  await expect(page.getByRole("heading", { name: monitorName })).toBeVisible();
  await expect(page.getByText("Core · heartbeat")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Latest Heartbeat", exact: true })).toBeVisible();
  await expect(page.getByText("status=ok")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Latest Heartbeat Failure" })).toBeVisible();
  await expect(page.getByText("[redacted]").first()).toBeVisible();
  await expect(page.locator("body")).not.toContainText("super-secret");
  await expect(page.locator("body")).not.toContainText("raw-token-value");
  await expect(page.getByRole("tab", { name: "Configuration" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Heartbeat Setup" })).toBeVisible();
  await expect(
    page.getByText("The token is shown after heartbeat monitor creation."),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Check history" }).click();
  await page.getByRole("row", { name: /status=ok/ }).click();
  await expect(page.getByRole("dialog")).toContainText("Heartbeat");
  await expect(page.getByRole("dialog")).toContainText("status=ok");
  await page.keyboard.press("Escape");

  await page.getByRole("link", { name: "View latest incident" }).click();
  await expect(page.getByRole("heading", { name: new RegExp(monitorName) })).toBeVisible();
  await expect(page.getByText("[redacted]").first()).toBeVisible();
  await expect(page.locator("body")).not.toContainText("super-secret");
  await expect(page.locator("body")).not.toContainText("raw-token-value");
  await page
    .getByRole("row", { name: /\[redacted\]/ })
    .first()
    .click();
  await expect(page.getByRole("dialog")).toContainText("Heartbeat");
  await expect(page.getByRole("dialog")).toContainText("[redacted]");
  await page.keyboard.press("Escape");

  await page.getByRole("link", { name: "View monitor" }).click();
  await page.getByRole("button", { name: "Delete" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByRole("heading", { name: "Monitors" })).toBeVisible();
});

test("exercises incident detail tabs and lifecycle actions", async ({ page }) => {
  await signIn(page);

  await page.goto("/incidents/seed-incident-seed-monitor-seed-agent-03-down-http");
  await expect(page.getByRole("heading", { name: "Down Server HTTP API is down" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Acknowledge" })).toBeVisible();
  await expect(page.getByText("Cause / Evidence")).toBeVisible();
  await page.getByRole("button", { name: "Inspect report" }).first().click();
  await expect(page.getByRole("dialog").getByText("Monitor Report")).toBeVisible();
  await expect(page.getByRole("dialog").getByText("Raw JSON")).toBeVisible();
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
  await expect(page.getByRole("button", { name: "Acknowledge" })).toBeVisible();
  await expect(page.getByText("Incident reopened").first()).toBeVisible();

  await page.getByRole("button", { name: "Acknowledge" }).click();
  await expect(page.getByRole("button", { name: "Acknowledge" })).toBeHidden();
  await expect(page.getByRole("button", { name: "Resolve" })).toBeVisible();

  await page.getByRole("button", { name: "Resolve" }).click();
  await expect(page.getByRole("button", { name: "Resolve" })).toBeHidden();
  await expect(page.getByText("Incident manually resolved").first()).toBeVisible();
});
