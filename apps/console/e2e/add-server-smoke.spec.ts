import { expect, test } from "@playwright/test";
import { coreApiURL, signIn } from "./console-smoke-helpers";

test("add server page shows the installer and detects a newly registered server", async ({
  page,
}) => {
  const machineId = `e2e-machine-${Date.now()}`;
  await signIn(page);
  await page.goto("/servers");
  await page.getByRole("link", { name: "Add server" }).first().click();
  await expect(page.getByRole("heading", { name: "Add server" })).toBeVisible();
  await expect(page.getByLabel("Install command")).toContainText("orion-agent-installer.sh");
  await expect(page.getByText("Waiting for a new server to appear...")).toBeVisible();

  const response = await page.request.post(`${coreApiURL}/register`, {
    data: { machine_id: machineId, name: `e2e-host-${machineId}`, os: "linux", arch: "amd64" },
  });
  expect(response.ok()).toBeTruthy();
  await expect(page.getByTestId("server-registered")).toBeVisible({ timeout: 15_000 });
});
