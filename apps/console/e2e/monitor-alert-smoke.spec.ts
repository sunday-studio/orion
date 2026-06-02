import { expect, test } from "@playwright/test";
import {
  authHeaders,
  coreApiURL,
  createWebhookDestination,
  sendWebhookTest,
  signIn,
  webhookReceiverURL,
} from "./console-smoke-helpers";

test.describe.configure({ mode: "serial" });

test("creates and manages a Core HTTP monitor", async ({ page }) => {
  const monitorName = `Core E2E HTTP ${Date.now()}`;
  const updatedName = `${monitorName} updated`;
  const monitorURL = "http://127.0.0.1:19080/health";

  await signIn(page);
  await page.getByRole("link", { name: "Monitors" }).click();
  await page.getByRole("button", { name: "Core monitor" }).click();
  await page.getByLabel("Name").fill(monitorName);
  await page.getByLabel("URL").fill(monitorURL);
  await page.getByRole("spinbutton", { name: "Expected status" }).fill("503");
  await page.getByLabel("Interval seconds").fill("45");
  await page.getByRole("button", { name: "Create", exact: true }).click();

  await page.getByPlaceholder("Search monitors").fill(monitorName);
  await expect(page.getByRole("link", { name: monitorName })).toBeVisible({ timeout: 15_000 });
  await page.getByRole("link", { name: monitorName }).click();
  await expect(page.getByRole("heading", { name: monitorName })).toBeVisible();
  await expect(page.getByText("Core · http")).toBeVisible();

  await page.getByRole("tab", { name: "Configuration" }).click();
  await expect(page.getByText(monitorURL)).toBeVisible();
  await expect(page.getByLabel("Configuration").getByText("45s")).toBeVisible();
  await page.getByRole("button", { name: "Test" }).click();
  await expect(page.getByText(/Core monitor test reported down:/)).toBeVisible();

  await page.reload();
  await expect(page.getByRole("heading", { name: monitorName })).toBeVisible();
  await page.getByRole("tab", { name: "Check history" }).click();
  await page
    .getByRole("row", { name: /unexpected HTTP status 200|expected 503|down/ })
    .first()
    .click();
  await expect(page.getByRole("dialog")).toContainText("Monitor Report");
  await expect(page.getByRole("dialog")).toContainText("expected_status");
  await expect(page.getByRole("dialog")).toContainText("503");
  await page.keyboard.press("Escape");

  await page.getByRole("tab", { name: "Incidents" }).click();
  await expect(page.getByRole("link", { name: new RegExp(monitorName) })).toBeVisible();
  await page.getByRole("button", { name: "Pause" }).click();
  await expect(page.getByText("Core monitor paused.")).toBeVisible();
  await page.getByRole("button", { name: "Resume" }).click();
  await expect(page.getByText("Core monitor resumed.")).toBeVisible();

  await page.getByRole("button", { name: "Edit" }).click();
  await page.getByLabel("Name").fill(updatedName);
  await page.getByRole("spinbutton", { name: "Expected status" }).fill("200");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("heading", { name: updatedName })).toBeVisible();
  await page.getByRole("button", { name: "Test" }).click();
  await expect(page.getByText("Core monitor test reported up.")).toBeVisible();
  await page.reload();
  await expect(page.getByRole("heading", { name: updatedName })).toBeVisible();

  await page.getByRole("button", { name: "Delete" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByRole("heading", { name: "Monitors" })).toBeVisible();
});

test("rejects unsupported Core monitor types through the browser API harness", async ({ page }) => {
  await signIn(page);

  const response = await page.request.post(`${coreApiURL}/monitors`, {
    data: {
      config: {},
      kind: "coffee",
      name: `Unsupported Core E2E ${Date.now()}`,
      type: "coffee",
    },
    headers: await authHeaders(page),
  });
  expect(response.status()).toBe(400);
  expect(await response.text()).toContain("Unsupported core monitor type");
});

test("creates webhook alert destinations and records sanitized delivery logs", async ({ page }) => {
  const stamp = Date.now();
  const sentDestination = `e2e-webhook-sent-${stamp}`;
  const failedDestination = `e2e-webhook-failed-${stamp}`;
  const secretToken = "super-secret-failure-token";

  await signIn(page);
  const resetCaptures = await page.request.delete(`${webhookReceiverURL}/captures`);
  expect(resetCaptures.ok()).toBeTruthy();
  await page.getByRole("link", { name: "Alerts" }).click();
  await page.getByRole("tab", { name: "Channels" }).click();
  await expect(page.getByRole("heading", { name: "Webhook Channels" })).toBeVisible();

  await createWebhookDestination(page, sentDestination, `${webhookReceiverURL}/webhook/success`);
  await createWebhookDestination(
    page,
    failedDestination,
    `${webhookReceiverURL}/webhook/failure?token=${secretToken}`,
  );

  await page.getByRole("tab", { name: "Rules" }).click();
  const ruleName = `e2e monitor failure ${stamp}`;
  await page.getByRole("button", { name: "New rule" }).click();
  const ruleDialog = page.getByRole("dialog", { name: "New alert rule" });
  await ruleDialog.getByLabel("Name", { exact: true }).fill(ruleName);
  for (const destination of [sentDestination, failedDestination]) {
    const checkbox = ruleDialog.getByRole("checkbox", { name: destination });
    if ((await checkbox.getAttribute("aria-checked")) !== "true") await checkbox.click();
  }
  await ruleDialog.getByRole("button", { name: "Create rule" }).click();
  const ruleRow = page.getByRole("row", { name: new RegExp(ruleName) });
  await expect(ruleRow).toContainText(sentDestination);
  await expect(ruleRow).toContainText(failedDestination);

  await page.getByRole("tab", { name: "Channels" }).click();
  await sendWebhookTest(page, sentDestination);
  await expect(
    page.getByText(`Test sent to ${sentDestination}. Delivery status: sent.`),
  ).toBeVisible();
  const capturesResponse = await page.request.get(`${webhookReceiverURL}/captures`);
  expect(capturesResponse.ok()).toBeTruthy();
  const captures = (await capturesResponse.json()) as {
    captures: { body: string; path: string }[];
  };
  expect(captures.captures.some((capture) => capture.path === "/webhook/success")).toBeTruthy();
  expect(captures.captures.map((capture) => capture.body).join("\n")).toContain(
    "Alert channel test",
  );

  await page.goto(
    `/alerts?tab=logs&status=sent&type=webhook&event_type=test&channel=${encodeURIComponent(sentDestination)}`,
  );
  await expect(page.getByRole("heading", { name: "Notification Log" })).toBeVisible();
  await expect(page.getByRole("row", { name: new RegExp(sentDestination) })).toContainText("sent");
  await page.getByRole("tab", { name: "Channels" }).click();
  await sendWebhookTest(page, failedDestination);
  await expect(
    page.getByText(`Test sent to ${failedDestination}. Delivery status: failed.`),
  ).toBeVisible();
  await page.goto(
    `/alerts?tab=logs&status=failed&type=webhook&event_type=test&channel=${encodeURIComponent(failedDestination)}`,
  );
  await expect(page.getByRole("row", { name: new RegExp(failedDestination) })).toContainText(
    "delivery failed; check Core logs",
  );
  await expect(page.locator("body")).not.toContainText(secretToken);
});

test("creates a Core heartbeat monitor and shows setup affordances", async ({ page }) => {
  const monitorName = `Core E2E Heartbeat ${Date.now()}`;

  await signIn(page);
  await page.getByRole("link", { name: "Monitors" }).click();
  await page.getByRole("button", { name: "Core monitor" }).click();
  await page.getByLabel("Name").fill(monitorName);
  await page.getByLabel("Core monitor type").selectOption("heartbeat");
  await expect(page.getByLabel("URL")).toBeHidden();
  await page.getByRole("spinbutton", { name: "Interval seconds" }).fill("90");
  await page.getByRole("spinbutton", { name: "Grace seconds" }).fill("30");
  await page.getByRole("button", { name: "Create" }).click();

  await expect(page.getByText("Heartbeat monitor created.")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Heartbeat Setup" })).toBeVisible();
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

  expect(
    (
      await page.request.post(failureEndpoint, {
        data: "password=super-secret token=raw-token-value",
      })
    ).ok(),
  ).toBeTruthy();
  expect((await page.request.post(successEndpoint, { data: "status=ok" })).ok()).toBeTruthy();
  await page.getByRole("link", { name: "Open monitor" }).click();
  await expect(page.getByRole("heading", { name: monitorName })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Latest Heartbeat", exact: true })).toBeVisible();
  await expect(page.getByText("[redacted]").first()).toBeVisible();
  await expect(page.locator("body")).not.toContainText("super-secret");
  await expect(page.locator("body")).not.toContainText("raw-token-value");

  await page.getByRole("button", { name: "Delete" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByRole("heading", { name: "Monitors" })).toBeVisible();
});
