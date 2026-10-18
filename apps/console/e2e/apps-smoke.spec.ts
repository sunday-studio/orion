import { expect, test } from "@playwright/test";
import { coreApiURL, signIn } from "./console-smoke-helpers";

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

test("an ingested app log appears in Service logs", async ({ page }) => {
  const appName = `E2E logs ${Date.now()}`;
  const marker = `e2e-marker-${Date.now()}`;
  await signIn(page);
  await page.goto("/settings/apps");
  await page.getByRole("button", { name: "New app" }).click();
  await page.getByLabel("Name").fill(appName);
  await page.getByRole("button", { name: "Create app" }).click();

  const drawer = page.getByRole("dialog", { name: appName });
  await drawer.getByLabel("Key name").fill("ingest");
  await drawer.getByRole("button", { name: "Create key" }).click();
  const secret = (await drawer.getByTestId("ingest-key-secret").innerText()).trim();

  const response = await page.request.post(`${coreApiURL}/ingest/logs`, {
    headers: { Authorization: `Bearer ${secret}` },
    data: { body: marker, severity: "error", event: "e2e.checked" },
  });
  expect(response.ok()).toBeTruthy();

  await page.goto("/logs");
  await page.getByPlaceholder("Search logs").fill(marker);
  await expect(page.getByRole("cell", { name: "e2e.checked" })).toBeVisible();
});

test("connect-first-app flow detects the first log", async ({ page }) => {
  await signIn(page);
  await page.goto("/apps/new");
  await expect(page.getByRole("heading", { name: "Connect your first app" })).toBeVisible();
  await page.getByLabel("Name").fill(`Connect ${Date.now()}`);
  await page.getByRole("button", { name: "Create app" }).click();

  await expect(page.getByRole("heading", { name: /Add the key to/ })).toBeVisible();
  await expect(page.getByRole("tabpanel")).toContainText("/v1/ingest/logs");
  const secret = (await page.getByLabel("Ingest key").innerText()).trim();
  await page.getByRole("button", { name: "I sent one" }).click();
  await expect(page.getByText("No log received yet")).toBeVisible();

  const response = await page.request.post(`${coreApiURL}/ingest/logs`, {
    headers: { Authorization: `Bearer ${secret}` },
    data: { body: "hello from e2e", severity: "info" },
  });
  expect(response.ok()).toBeTruthy();
  await expect(page.getByText("Its first log just arrived.")).toBeVisible({ timeout: 15_000 });
  await expect(page.getByRole("link", { name: "Open in Logs" })).toBeVisible();
});
