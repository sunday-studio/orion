import { expect, test } from "@playwright/test";
import { coreApiURL, signIn } from "./console-smoke-helpers";

test("add server mints an enrollment and detects the node registering with it", async ({
  page,
}) => {
  const machineId = `e2e-machine-${Date.now()}`;
  await signIn(page);
  await page.goto("/servers");
  await page.getByRole("link", { name: "Add server" }).first().click();
  await expect(page.getByRole("heading", { name: "Add a server" })).toBeVisible();

  const command = page.getByLabel("Install command");
  await expect(command).toContainText("ORION_ENROLLMENT_TOKEN");
  const text = (await command.innerText()).trim();
  const token = /ORION_ENROLLMENT_TOKEN='([^']+)'/.exec(text)?.[1] ?? "";
  expect(token).not.toBe("");
  await expect(page.getByText("Waiting for the node")).toBeVisible();

  const response = await page.request.post(`${coreApiURL}/register`, {
    headers: { "X-Orion-Enrollment-Token": token },
    data: { machine_id: machineId, name: `e2e-host-${machineId}`, os: "linux", arch: "amd64" },
  });
  expect(response.ok()).toBeTruthy();
  await expect(page.getByText("The command will:")).toBeVisible({
    timeout: 15_000,
  });
});
