import { expect, test } from "@playwright/test";
import { signIn } from "./console-smoke-helpers";

test("creates an app, reveals an ingest key once, and revokes it", async ({ page }) => {
  const appName = `E2E app ${Date.now()}`;
  await signIn(page);
  await page.goto("/settings/apps");
  await expect(page.getByRole("heading", { name: "Apps" })).toBeVisible();

  await page.getByRole("button", { name: "New app" }).click();
  await page.getByLabel("Name").fill(appName);
  await page.getByRole("button", { name: "Create app" }).click();

  const drawer = page.getByRole("dialog", { name: appName });
  await expect(drawer).toBeVisible();
  await drawer.getByLabel("Key name").fill("production");
  await drawer.getByRole("button", { name: "Create key" }).click();

  const secret = drawer.getByTestId("ingest-key-secret");
  await expect(secret).not.toBeEmpty();
  const done = drawer.getByRole("button", { name: "Done" });
  await expect(done).toBeDisabled();
  await drawer.getByRole("checkbox", { name: "I stored the key" }).click();
  await done.click();
  await expect(drawer.getByTestId("ingest-key-secret")).toHaveCount(0);

  await expect(drawer.getByRole("listitem").first()).toContainText("production");
  await drawer.getByRole("button", { name: "Revoke" }).click();
  await page
    .getByRole("dialog", { name: "Revoke production?" })
    .getByRole("button", { name: "Revoke key" })
    .click();
  await expect(drawer.getByText("revoked")).toBeVisible();
});
