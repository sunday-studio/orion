import { expect, test } from "@playwright/test";
import { signIn } from "./console-smoke-helpers";

test("creates an email channel and adds a pending recipient", async ({ page }) => {
  const name = `e2e-email-${Date.now()}`;
  await signIn(page);
  await page.goto("/alerts?tab=channels");
  await page.getByRole("tab", { name: /Channels/ }).click();

  await page.getByRole("button", { name: "New email channel" }).click();
  await page.getByLabel("Name").fill(name);
  await page.getByRole("button", { name: "Create channel" }).click();

  const drawer = page.getByRole("dialog", { name });
  await expect(drawer).toBeVisible();
  await drawer.getByLabel("Recipient email").fill("oncall@example.com");
  await drawer.getByRole("button", { name: "Add recipient" }).click();
  await expect(drawer.getByText("oncall@example.com")).toBeVisible();
  await expect(drawer.getByText("pending")).toBeVisible();
});
