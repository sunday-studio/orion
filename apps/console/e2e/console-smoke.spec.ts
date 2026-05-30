import { type Page, expect, test } from "@playwright/test";

const username = "admin";
const password = "change-me";
const webhookReceiverURL = "http://127.0.0.1:19080";

test.describe.configure({ mode: "serial" });

const signIn = async (page: Page) => {
  await page.goto("/login");
  await page.getByPlaceholder("Username").fill(username);
  await page.getByPlaceholder("Password").fill(password);
  await page.getByRole("button", { name: "Enter" }).click();
  await expect(page.getByRole("heading", { name: "Incidents" })).toBeVisible();
};

const createWebhookDestination = async (page: Page, name: string, url: string) => {
  await page.getByRole("button", { name: "New webhook" }).click();
  const dialog = page.getByRole("dialog", { name: "New webhook" });
  await dialog.getByPlaceholder("ops-webhook").fill(name);
  await dialog.getByPlaceholder("https://example.com/webhook").fill(url);
  await dialog.getByRole("button", { name: "Create destination" }).click();
  await expect(page.getByRole("row", { name: new RegExp(name) })).toBeVisible();
};

const sendWebhookTest = async (page: Page, name: string) => {
  await page.getByLabel(`Open actions for ${name}`).click();
  await page.getByRole("menuitem", { name: "Send test" }).click();
};

const openSettings = async (page: Page) => {
  await signIn(page);
  await page.getByRole("link", { name: "Settings" }).click();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
};

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

  await page.getByRole("link", { name: "Servers" }).click();
  await expect(page.getByRole("heading", { name: "Servers" })).toBeVisible();
  await expect(page.getByText("Healthy Server", { exact: true })).toBeVisible();
  await expect(page.getByText("9 monitors").first()).toBeVisible();

  await page.getByRole("link", { name: "Monitors" }).click();
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

  await page.getByRole("link", { name: "Alerts" }).click();
  await expect(page.getByRole("heading", { name: "Alerts" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "Notification Log" })).toBeVisible();
  await page.getByRole("tab", { name: "Rules" }).click();
  await expect(page.getByRole("heading", { name: "Rules" })).toBeVisible();

  await page.getByRole("link", { name: "Status" }).click();
  await expect(page.getByRole("heading", { name: "Status Pages" })).toBeVisible();

  await page.getByRole("link", { name: "Logs" }).click();
  await expect(page.getByRole("heading", { name: "Logs" })).toBeVisible();
  await expect(page.getByPlaceholder("Search events")).toBeVisible();

  await page.getByRole("link", { name: "Settings" }).click();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  await expect(page.getByLabel("Raw report days")).toHaveValue("30");
  await expect(
    page.getByRole("checkbox", { name: "Archive raw reports automatically" }),
  ).toBeChecked();
  await expect(page.getByRole("checkbox", { name: "Enable rollups" })).toBeChecked();
});

test("reads, saves, validates, and runs Settings lifecycle controls", async ({ page }) => {
  await signIn(page);

  await page.getByRole("link", { name: "Settings" }).click();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();

  const rawReportDays = page.getByLabel("Raw report days");
  const rollupDays = page.getByLabel("Rollup days");
  const archiveDir = page.getByLabel("Archive directory");
  const archiveSchedule = page.getByLabel("Archive schedule");

  await expect(rawReportDays).toHaveValue("30");
  await expect(rollupDays).toHaveValue("");
  await expect(archiveDir).not.toHaveValue("");
  await expect(archiveSchedule).toContainText("Manual only");
  await expect(page.getByText("Last archive")).toBeVisible();
  await expect(page.getByText("success")).toBeVisible();

  await rawReportDays.fill("0");
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText("Fix the highlighted settings before saving.")).toBeVisible();

  await rawReportDays.fill("45");
  await rollupDays.fill("120");
  await archiveSchedule.click();
  await page.getByRole("option", { name: "Daily" }).click();
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText("Settings saved.")).toBeVisible();
  await expect(rawReportDays).toHaveValue("45");
  await expect(rollupDays).toHaveValue("120");
  await expect(archiveSchedule).toContainText("Daily");

  await page.reload();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  await expect(page.getByLabel("Raw report days")).toHaveValue("45");
  await expect(page.getByLabel("Rollup days")).toHaveValue("120");
  await expect(page.getByLabel("Archive schedule")).toContainText("Daily");

  await page.getByRole("button", { name: "Run rollup" }).click();
  await expect(page.getByText(/Rolled up \d+ reports for /)).toBeVisible();

  await page.getByRole("button", { name: "Run archive" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Run archive" }).click();
  await expect(page.getByText(/Archived \d+ reports\./)).toBeVisible();

  // Restore seed defaults so later serial settings tests start from a clean baseline.
  await rawReportDays.fill("30");
  await archiveSchedule.click();
  await page.getByRole("option", { name: "Manual only" }).click();
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText("Settings saved.")).toBeVisible();
  await expect(rawReportDays).toHaveValue("30");
});

test("confirms archive and prevents duplicate maintenance submissions", async ({ page }) => {
  await signIn(page);
  await page.getByRole("link", { name: "Settings" }).click();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();

  let rollupRequests = 0;
  await page.route("**/v1/settings/data-lifecycle/actions/rollup", async (route) => {
    rollupRequests += 1;
    await new Promise((resolve) => setTimeout(resolve, 300));
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          result: {
            date: "2026-05-28",
            monitor_days: 0,
            report_count: 0,
            skipped_today: false,
          },
        },
      }),
    });
  });

  await page.getByRole("button", { name: "Run rollup" }).evaluate((button: HTMLButtonElement) => {
    button.click();
    button.click();
  });
  await expect(page.getByRole("button", { name: "Running rollup..." })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Run archive" })).toBeDisabled();
  await expect(page.getByText("No monitor reports matched this rollup day.")).toBeVisible();
  expect(rollupRequests).toBe(1);

  let archiveRequests = 0;
  await page.route("**/v1/settings/data-lifecycle/actions/archive", async (route) => {
    archiveRequests += 1;
    await new Promise((resolve) => setTimeout(resolve, 300));
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          result: {
            archive_path: "/tmp/orion/archive/raw-reports-2026-05.sqlite",
            cutoff: "2026-05-01T00:00:00Z",
            agent_reports_archived: 0,
            monitor_reports_archived: 0,
            archive_raw_reports: true,
            skipped_because_disabled: false,
            skipped_because_no_reports: true,
          },
        },
      }),
    });
  });

  await page.getByRole("button", { name: "Run archive" }).click();
  await expect(page.getByRole("dialog")).toContainText("Archive raw reports");
  await expect(page.getByRole("dialog")).toContainText("Reports older than");
  await expect(page.getByRole("dialog")).toContainText("Archive destination");
  await expect(page.getByRole("dialog")).toContainText("move out of the hot Core database");

  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Run archive" })
    .evaluate((button: HTMLButtonElement) => {
      button.click();
      button.click();
    });
  await expect(page.getByRole("button", { name: "Running archive..." })).toBeDisabled();
  await expect(page.getByText("No raw reports matched the cutoff.")).toBeVisible();
  await expect(page.getByText("Last rollup")).toBeVisible();
  await expect(page.getByText("Last archive")).toBeVisible();
  expect(archiveRequests).toBe(1);
});

test("navigates from incident list rows to incident detail", async ({ page }) => {
  await signIn(page);

  await page.goto("/incidents?agent=seed-agent-03-down");
  await expect(page.getByRole("heading", { name: "Incidents" })).toBeVisible();

  const incidentLink = page.getByRole("link", {
    name: "Down Server HTTP API is down",
  });
  await expect(incidentLink).toBeVisible();
  await incidentLink.click();

  await expect(page).toHaveURL(/\/incidents\/seed-incident-seed-monitor-seed-agent-03-down-http/);
  await expect(page.getByRole("heading", { name: "Down Server HTTP API is down" })).toBeVisible();
  await expect(page.getByRole("link", { name: "View server" })).toBeVisible();
  await expect(page.getByRole("link", { name: "View monitor" })).toBeVisible();
});

test("creates and manages a Core HTTP monitor", async ({ page }) => {
  const monitorName = `Core E2E HTTP ${Date.now()}`;
  const updatedName = `${monitorName} updated`;

  await signIn(page);

  await page.getByRole("link", { name: "Monitors" }).click();
  await page.getByRole("button", { name: "Core monitor" }).click();
  await page.getByLabel("Name").fill(monitorName);
  await page.getByLabel("URL").fill("https://example.com/health");
  await page.getByLabel("Expected status").fill("200");
  await page.getByLabel("Interval seconds").fill("45");
  await page.getByRole("button", { name: "Create", exact: true }).click();

  await page.getByPlaceholder("Search monitors").fill(monitorName);
  await expect(page.getByRole("link", { name: monitorName })).toBeVisible({ timeout: 15_000 });
  await page.getByRole("link", { name: monitorName }).click();
  await expect(page.getByRole("heading", { name: monitorName })).toBeVisible();
  await expect(page.getByText("Core · http")).toBeVisible();

  await page.getByRole("tab", { name: "Configuration" }).click();
  await expect(page.getByText("https://example.com/health")).toBeVisible();
  await expect(page.getByLabel("Configuration").getByText("45s")).toBeVisible();

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

test("creates webhook alert destinations and records sanitized delivery logs", async ({ page }) => {
  const stamp = Date.now();
  const sentDestination = `e2e-webhook-sent-${stamp}`;
  const failedDestination = `e2e-webhook-failed-${stamp}`;
  const secretToken = "super-secret-failure-token";

  await signIn(page);
  await page.request.delete(`${webhookReceiverURL}/captures`);

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
  await expect(page.getByRole("heading", { name: "Rules" })).toBeVisible();
  const ruleName = `e2e monitor failure ${stamp}`;
  await page.getByRole("button", { name: "New rule" }).click();
  const ruleDialog = page.getByRole("dialog", { name: "New alert rule" });
  await ruleDialog.getByLabel("Name", { exact: true }).fill(ruleName);
  for (const destination of [sentDestination, failedDestination]) {
    const checkbox = ruleDialog.getByRole("checkbox", { name: destination });
    if ((await checkbox.getAttribute("aria-checked")) !== "true") {
      await checkbox.click();
    }
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
    `/alerts?tab=logs&status=sent&type=webhook&event_type=test&channel=${encodeURIComponent(
      sentDestination,
    )}`,
  );
  await expect(page.getByRole("heading", { name: "Notification Log" })).toBeVisible();
  await expect(page.getByRole("row", { name: new RegExp(sentDestination) })).toContainText("sent");
  await expect(page.getByRole("row", { name: new RegExp(sentDestination) })).toContainText(
    "webhook",
  );
  await expect(page.getByRole("row", { name: new RegExp(sentDestination) })).toContainText("test");

  await page.getByRole("tab", { name: "Channels" }).click();
  await sendWebhookTest(page, failedDestination);
  await expect(
    page.getByText(`Test sent to ${failedDestination}. Delivery status: failed.`),
  ).toBeVisible();

  await page.goto(
    `/alerts?tab=logs&status=failed&type=webhook&event_type=test&channel=${encodeURIComponent(
      failedDestination,
    )}`,
  );
  await expect(page.getByRole("heading", { name: "Notification Log" })).toBeVisible();
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
  await expect(page.getByText("status=ok").first()).toBeVisible();
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
  await page.getByRole("row", { name: /Heartbeat up/ }).click();
  await expect(page.getByRole("dialog")).toContainText("Heartbeat");
  await expect(page.getByRole("dialog")).toContainText("status=ok");
  await page.keyboard.press("Escape");

  await page.getByRole("link", { name: "View incident" }).first().click();
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

test("reads and saves data lifecycle settings", async ({ page }) => {
  await openSettings(page);

  await expect(page.getByLabel("Raw report days")).toHaveValue("30");
  await expect(page.getByLabel("Archive directory")).toHaveValue(/\/archive$/);
  await expect(page.getByRole("combobox", { name: /Archive schedule/ })).toContainText(
    "Manual only",
  );
  await expect(
    page.getByRole("checkbox", { name: "Archive raw reports automatically" }),
  ).toBeChecked();
  await expect(page.getByRole("checkbox", { name: "Enable rollups" })).toBeChecked();

  await page.getByLabel("Raw report days").fill("31");
  await page.getByLabel("Rollup days").fill("45");
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText("Settings saved.")).toBeVisible();

  await page.reload();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  await expect(page.getByLabel("Raw report days")).toHaveValue("31");
  await expect(page.getByLabel("Rollup days")).toHaveValue("45");
  await expect(
    page.getByRole("checkbox", { name: "Archive raw reports automatically" }),
  ).toBeChecked();
  await expect(page.getByRole("checkbox", { name: "Enable rollups" })).toBeChecked();
});

test("validates settings and runs manual lifecycle actions", async ({ page }) => {
  await openSettings(page);

  await page.getByLabel("Raw report days").fill("0");
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText("Fix the highlighted settings before saving.")).toBeVisible();

  await page.getByLabel("Raw report days").fill("31");
  await page.getByLabel("Rollup days").fill("45");
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText("Settings saved.")).toBeVisible();

  await page.getByRole("button", { name: "Run rollup" }).click();
  await expect(page.getByText(/Rolled up \d+ reports for /)).toBeVisible();
  await expect(page.getByText("Last rollup")).toBeVisible();

  await page.getByRole("button", { name: "Run archive" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Run archive" }).click();
  await expect(page.getByText(/Archived \d+ reports\./)).toBeVisible();
  await expect(page.getByText("Last archive")).toBeVisible();
  await expect(page.getByText("success")).toBeVisible();
});
