import { expect, test } from "@playwright/test";
import { signIn } from "./console-smoke-helpers";

test("follows the device theme and updates live when it changes", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" });
  await signIn(page);
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");

  await page.emulateMedia({ colorScheme: "dark" });
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");

  await page.emulateMedia({ colorScheme: "light" });
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
});

test("a manual theme choice overrides the device and survives reloads", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await signIn(page);
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");

  await page.getByRole("button", { name: "Account menu" }).click();
  await page.getByRole("radio", { name: "Light" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");

  // The device stays dark but the explicit choice wins, also after a reload.
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");

  await page.getByRole("button", { name: "Account menu" }).click();
  await page.getByRole("radio", { name: "System" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
});

test("a legacy orion.theme=light value is not treated as a manual choice", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await page.addInitScript(() => localStorage.setItem("orion.theme", "light"));
  await signIn(page);
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.getByRole("button", { name: "Account menu" }).click();
  await expect(page.getByRole("radio", { name: "System" })).toHaveAttribute("aria-checked", "true");
  // The stale key is removed so it cannot come back as a choice.
  expect(await page.evaluate(() => localStorage.getItem("orion.theme"))).toBeNull();
});

test("the login page follows the device theme", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await page.goto("/login");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  const background = await page
    .locator("main")
    .evaluate((el) => getComputedStyle(el).backgroundColor);
  expect(background).toBe("rgb(15, 19, 17)");
});
