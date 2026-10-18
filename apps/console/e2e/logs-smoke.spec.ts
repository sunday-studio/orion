import { expect, test } from "@playwright/test";
import { signIn } from "./console-smoke-helpers";

test("keeps event log filters and service log filters in the URL", async ({ page }) => {
  await signIn(page);
  await page.goto("/logs?tab=events");
  await expect(page.getByRole("heading", { name: "Logs" })).toBeVisible();

  const search = page.getByPlaceholder("Search events");
  await search.fill("monitor");
  await expect(page).toHaveURL(/[?&]q=monitor/);

  await page.reload();
  await expect(page.getByPlaceholder("Search events")).toHaveValue("monitor");

  await page.getByRole("button", { name: "Clear" }).click();
  await expect(page).not.toHaveURL(/[?&]q=/);

  await page.getByRole("tab", { name: "Internal logs" }).click();
  await page.getByPlaceholder("Component").fill("core");
  await expect(page).toHaveURL(/serviceComponent=core/);
});

test("opens an event detail drawer from an event row", async ({ page }) => {
  await signIn(page);
  await page.goto("/logs?tab=events");
  const firstRow = page.locator("tbody tr").first();
  await expect(firstRow.getByRole("cell").first()).not.toBeEmpty();
  await firstRow.click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(page.getByLabel("Raw event")).toContainText('"id"');
});

test("shows the unified Service logs tab with a one hour default window", async ({ page }) => {
  await signIn(page);
  await page.goto("/logs");
  await expect(page.getByRole("tab", { name: "Service logs" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(page.getByRole("combobox", { name: "Time" })).toContainText("Last hour");
  await expect(page.getByRole("tab", { name: "Status logs" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "Internal logs" })).toBeVisible();
  await expect(page.getByPlaceholder("Search logs")).toBeVisible();
});
