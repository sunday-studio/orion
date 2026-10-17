import { expect, test } from "@playwright/test";
import { signIn } from "./console-smoke-helpers";

test.describe.configure({ mode: "serial" });

test.beforeEach(async ({ page }) => {
  await signIn(page);
  await page.goto("/design-system");
  await expect(page.getByRole("heading", { name: "Design system" })).toBeVisible();
});

test("dialog opens, traps focus and closes with Escape", async ({ page }) => {
  await page.getByRole("button", { name: "Open dialog" }).click();
  await expect(page.getByRole("dialog", { name: "Example dialog" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Open dialog" })).toBeFocused();
});

test("confirm dialog names the blast radius and shows a visible confirm button", async ({
  page,
}) => {
  await page.getByRole("button", { name: "Delete server" }).click();
  const dialog = page.getByRole("dialog", { name: "Delete edge-01?" });
  await expect(dialog.getByText("4 monitors on this server will be deleted.")).toBeVisible();
  const confirm = dialog.getByRole("button", { name: "Delete server" });
  await expect(confirm).toBeVisible();
  const background = await confirm.evaluate((el) => getComputedStyle(el).backgroundColor);
  expect(background).not.toBe("rgba(0, 0, 0, 0)");
  await confirm.click();
  await expect(page.getByText("Confirmed 1")).toBeVisible();
});

test("tabs switch with the arrow keys", async ({ page }) => {
  await page.getByRole("tab", { name: "First" }).focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("tab", { name: "Second" })).toHaveAttribute("aria-selected", "true");
  await expect(page.getByText("Second panel")).toBeVisible();
});

test("checkbox and switch toggle with the keyboard", async ({ page }) => {
  const checkbox = page.getByRole("checkbox", { name: "Notify me" });
  await checkbox.focus();
  await page.keyboard.press("Space");
  await expect(checkbox).toBeChecked();

  const toggle = page.getByRole("switch", { name: "Enabled" });
  await toggle.focus();
  await page.keyboard.press("Space");
  await expect(page.getByText("Enabled: on")).toBeVisible();
});

test("select chooses an option with the keyboard", async ({ page }) => {
  const trigger = page.getByRole("combobox", { name: "Status" }).first();
  await trigger.focus();
  await page.keyboard.press("Enter");
  await page.getByRole("option", { name: "Down" }).first().click();
  await expect(trigger).toContainText("Down");
});

test("combobox filters options as you type", async ({ page }) => {
  const input = page.getByRole("combobox", { name: "Server" });
  await input.fill("db");
  await expect(page.getByRole("option", { name: "db-01" })).toBeVisible();
  await expect(page.getByRole("option", { name: "edge-01" })).toHaveCount(0);
});

test("menu opens from the keyboard and closes with Escape", async ({ page }) => {
  const trigger = page.getByRole("button", { name: "Open menu" });
  await trigger.focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("menuitem", { name: "Rename" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("menuitem", { name: "Rename" })).toHaveCount(0);
});

test("tooltip appears on keyboard focus", async ({ page }) => {
  await page.getByRole("button", { name: "Hover or focus me" }).focus();
  await expect(page.getByText("Helpful hint")).toBeVisible();
});

test("drawer and popover open and close", async ({ page }) => {
  await page.getByRole("button", { name: "Open drawer" }).click();
  await expect(page.getByRole("dialog", { name: "Example drawer" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);

  await page.getByRole("button", { name: "Open popover" }).click();
  await expect(page.getByText("Popover content")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByText("Popover content")).toHaveCount(0);
});

test("toast appears and can be dismissed", async ({ page }) => {
  await page.getByRole("button", { name: "Show toast" }).click();
  await expect(page.getByText("Your changes were saved.")).toBeVisible();
  // Base UI reveals the close control once the toast region is hovered or focused.
  await page.getByText("Your changes were saved.").hover();
  await page.getByRole("button", { name: "Dismiss" }).click();
  await expect(page.getByText("Your changes were saved.")).toHaveCount(0);
});

test("data table filters via URL state, opens rows with Enter and shows skeletons", async ({
  page,
}) => {
  await page.getByPlaceholder("Search monitors").fill("db");
  await expect(page).toHaveURL(/dsQ=db/);
  await expect(page.getByRole("cell", { name: "db-primary" })).toBeVisible();
  await expect(page.getByRole("cell", { name: "api-https" })).toHaveCount(0);

  await page.getByRole("row", { name: /db-primary/ }).focus();
  await page.keyboard.press("Enter");
  await expect(page.getByText("Last row opened: db-primary")).toBeVisible();

  await page.getByPlaceholder("Search monitors").fill("zzz");
  await expect(page.getByText("No monitors match the current filters.")).toBeVisible();
  await page.getByRole("button", { name: "Reset filters" }).click();
  await expect(page.getByRole("cell", { name: "api-https" })).toBeVisible();
});

test("theme switches to dark and sets data-theme", async ({ page }) => {
  await page.getByRole("combobox", { name: "Theme" }).click();
  await page.getByRole("option", { name: "Dark" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.getByRole("combobox", { name: "Theme" }).click();
  await page.getByRole("option", { name: "Light" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
});
