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
