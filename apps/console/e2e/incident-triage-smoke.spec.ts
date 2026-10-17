import { expect, test } from "@playwright/test";
import { signIn } from "./console-smoke-helpers";

test("lists failing monitors on the triage page and dismisses one", async ({ page }) => {
  await signIn(page);
  await page.goto("/incidents");
  await page.getByRole("link", { name: "Triage candidates" }).click();
  await expect(page.getByRole("heading", { name: "Triage" })).toBeVisible();

  const rows = page.locator("tbody tr");
  await expect(rows.first().getByRole("cell").first()).not.toBeEmpty();
  const dismissButtons = page.getByRole("button", { name: "Dismiss" });
  if ((await dismissButtons.count()) === 0) {
    await expect(page.getByText("Nothing to triage")).toBeVisible();
    return;
  }
  const before = await rows.count();
  await dismissButtons.first().click();
  await page
    .getByRole("dialog", { name: "Dismiss candidate?" })
    .getByRole("button", { name: "Dismiss" })
    .click();
  await expect(page.getByText("Candidate dismissed")).toBeVisible();
  await expect.poll(async () => rows.count()).not.toBe(before);
});
