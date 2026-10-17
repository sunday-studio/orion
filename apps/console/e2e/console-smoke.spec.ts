import { expect, test } from "@playwright/test";
import { password, signIn, username } from "./console-smoke-helpers";

test.describe.configure({ mode: "serial" });

test("signs in, rejects bad credentials, and signs out", async ({ page }) => {
  await page.goto("/login");
  await page.getByPlaceholder("Username").fill(username);
  await page.getByPlaceholder("Password").fill("wrong-password");
  await page.getByRole("button", { name: "Enter" }).click();
  await expect(page.getByText("Invalid credentials")).toBeVisible();

  await page.getByPlaceholder("Password").fill(password);
  await page.getByRole("button", { name: "Enter" }).click();
  await expect(page.getByRole("heading", { name: "Overview" })).toBeVisible();

  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page.getByPlaceholder("Username")).toBeVisible();
});

test("renders primary operations pages with seeded Core data", async ({ page }) => {
  await signIn(page);

  await page
    .getByLabel("Primary navigation")
    .getByRole("link", { name: "Servers", exact: true })
    .click();
  await expect(page.getByRole("heading", { name: "Servers" })).toBeVisible();
  await expect(page.getByText("Healthy Server", { exact: true })).toBeVisible();
  await expect(page.getByText("9 monitors").first()).toBeVisible();

  await page
    .getByLabel("Primary navigation")
    .getByRole("link", { name: "Monitors", exact: true })
    .click();
  await expect(page.getByRole("heading", { name: "Monitors" })).toBeVisible();
  await page.getByPlaceholder("Search monitors").fill("Healthy Server HTTP API");
  await expect(page.getByText("Healthy Server HTTP API")).toBeVisible();

  await page.goto("/monitors?owner=core&type=http&source=core");
  await expect(page.getByRole("heading", { name: "Monitors" })).toBeVisible();
  await expect(page.getByText("Core Public API")).toBeVisible();
  await expect(page.getByRole("row", { name: /Core Public API.*http.*Core/ })).toBeVisible();
  const workerDiagnostics = page.getByLabel("Core worker diagnostics");
  await expect(
    workerDiagnostics.getByRole("heading", { name: "Core monitor worker" }),
  ).toBeVisible();
  await expect(workerDiagnostics.getByText("unknown", { exact: true }).first()).toBeVisible();
  await expect(
    page.getByText("No Core monitor worker heartbeat is available.").first(),
  ).toBeVisible();
  await expect(page.getByText("Worker attention needed").first()).toBeVisible();
  await page.getByRole("link", { name: "Core Public API" }).click();
  await expect(page.getByRole("heading", { name: "Core Public API" })).toBeVisible();
  await expect(page.getByText("Core · http")).toBeVisible();
  await expect(page.getByText("No Core monitor worker heartbeat is available.")).toBeVisible();

  await page
    .getByLabel("Primary navigation")
    .getByRole("link", { name: "Alerts", exact: true })
    .click();
  await expect(page.getByRole("heading", { name: "Alerts" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "Notification Log" })).toBeVisible();
  await page.getByRole("tab", { name: "Rules" }).click();
  await expect(page.getByRole("heading", { name: "Rules" })).toBeVisible();

  await page
    .getByLabel("Primary navigation")
    .getByRole("link", { name: "Status", exact: true })
    .click();
  await expect(page.getByRole("heading", { name: "Status Pages" })).toBeVisible();

  await page
    .getByLabel("Primary navigation")
    .getByRole("link", { name: "Logs", exact: true })
    .click();
  await expect(page.getByRole("heading", { name: "Logs" })).toBeVisible();
  await expect(page.getByPlaceholder("Search events")).toBeVisible();

  await page
    .getByLabel("Primary navigation")
    .getByRole("link", { name: "Settings", exact: true })
    .click();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  await expect(page.getByLabel("Raw report days")).toHaveValue("30");
  await expect(
    page.getByRole("checkbox", { name: "Archive raw reports automatically" }),
  ).toBeChecked();
  await expect(page.getByRole("checkbox", { name: "Enable rollups" })).toBeChecked();
});

test("redirects unauthenticated visits to login and returns after sign in", async ({ page }) => {
  await page.goto("/monitors");
  await expect(page).toHaveURL(/\/login\?from=%2Fmonitors/);
  await page.getByPlaceholder("Username").fill(username);
  await page.getByPlaceholder("Password").fill(password);
  await page.getByRole("button", { name: "Enter" }).click();
  await expect(page).toHaveURL(/\/monitors$/);
});

test("renders a not-found page for unknown routes", async ({ page }) => {
  await signIn(page);
  await page.goto("/no-such-page");
  await expect(page.getByText("Page not found")).toBeVisible();
});

test("shows the overview landing page and dismisses onboarding", async ({ page }) => {
  await signIn(page);
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByLabel("Health summary")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Open incidents" })).toBeVisible();
  await expect(page.getByLabel("Getting started")).toBeVisible();
  await page.getByRole("button", { name: "Dismiss" }).click();
  await expect(page.getByLabel("Getting started")).toHaveCount(0);
  await page.reload();
  await expect(page.getByLabel("Getting started")).toHaveCount(0);
});

test("opens the command palette and shortcuts sheet from the keyboard", async ({ page }) => {
  await signIn(page);
  await page.keyboard.press("Control+k");
  await page.getByLabel("Search destinations").fill("mon");
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/\/monitors$/);
  await expect(page.getByLabel("Search destinations")).toHaveCount(0);

  await page.keyboard.press("?");
  await expect(page.getByRole("heading", { name: "Keyboard shortcuts" })).toBeVisible();
});
