import { expect, test } from "@playwright/test";
import { openSettings, signIn } from "./console-smoke-helpers";

test.describe.configure({ mode: "serial" });

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
