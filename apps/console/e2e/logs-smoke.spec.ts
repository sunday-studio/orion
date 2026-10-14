import { expect, test } from "@playwright/test";
import { signIn } from "./console-smoke-helpers";

test("keeps event log filters and service log filters in the URL", async ({ page }) => {
  await signIn(page);
  await page.goto("/logs");
  await expect(page.getByRole("heading", { name: "Logs" })).toBeVisible();

  const search = page.getByPlaceholder("Search events");
  await search.fill("monitor");
  await expect(page).toHaveURL(/[?&]q=monitor/);

  await page.reload();
  await expect(page.getByPlaceholder("Search events")).toHaveValue("monitor");

  await page.getByRole("button", { name: "Clear" }).click();
  await expect(page).not.toHaveURL(/[?&]q=/);

  await page.getByRole("tab", { name: "Service Logs" }).click();
  await page.getByPlaceholder("Component").fill("core");
  await expect(page).toHaveURL(/serviceComponent=core/);
});
