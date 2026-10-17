import { expect, test } from "@playwright/test";
import { mockApi } from "./support";

const created = {
  id: "enr-1",
  status: "waiting",
  token: "orn_enr_x",
  platform: "linux",
  core_url: "https://core.example.com",
  created_at: "2025-10-16T14:30:00Z",
  expires_at: "2025-10-16T14:45:00Z",
  install_command: "curl -fsSL https://core.example.com/install.sh | sudo sh",
};

const overview = {
  pattern: /\/v1\/overview$/,
  body: { state: "ok", ongoing_incident_count: 0, health_issue_count: 0, stats: {} },
};

test("login keeps the dark hero gradient off narrow screens", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 800 });
  await page.goto("/login");
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
  const display = await page.locator(".lg-bg").evaluate((el) => getComputedStyle(el).display);
  expect(display).toBe("none");
});

test("leaving Add server cancels an unused enrollment but not a claimed one", async ({ page }) => {
  const deletes: string[] = [];
  let status = "waiting";
  await mockApi(page, [
    overview,
    { pattern: /\/v1\/auth\/me$/, body: { username: "admin" } },
    { pattern: /\/v1\/enrollments$/, body: created },
    { pattern: /\/v1\/enrollments\/enr-1$/, body: () => ({ ...created, status }) },
  ]);
  page.on("request", (request) => {
    if (request.method() === "DELETE" && request.url().includes("/v1/enrollments/")) {
      deletes.push(request.url());
    }
  });

  await page.goto("/servers/new");
  await page.getByText("Waiting for the agent").waitFor();
  status = "claimed";
  await page.getByText("The command will:").waitFor({ timeout: 10_000 });
  await page.getByRole("link", { name: "Cancel" }).click();
  await page.waitForURL(/\/servers$/);
  await page.waitForTimeout(500);
  expect(deletes).toHaveLength(0);
});

test("Add server offers only Linux and macOS", async ({ page }) => {
  await mockApi(page, [
    overview,
    { pattern: /\/v1\/auth\/me$/, body: { username: "admin" } },
    { pattern: /\/v1\/enrollments$/, body: created },
    { pattern: /\/v1\/enrollments\/enr-1$/, body: created },
  ]);
  await page.goto("/servers/new");
  const radios = page.getByRole("radiogroup", { name: "Operating system" }).getByRole("radio");
  await expect(radios).toHaveCount(2);
  await expect(radios.nth(0)).toContainText("Linux");
  await expect(radios.nth(1)).toContainText("macOS");
});

test("Overview stays the active tab while connecting an app", async ({ page }) => {
  await mockApi(page, [overview, { pattern: /\/v1\/auth\/me$/, body: { username: "admin" } }]);
  await page.goto("/apps/new");
  const nav = page.getByLabel("Primary navigation");
  await expect(nav.getByRole("link", { name: "Overview" })).toHaveAttribute("aria-current", "page");
  await expect(nav.getByRole("link", { name: "Servers" })).not.toHaveAttribute(
    "aria-current",
    "page",
  );
});

test("the first-run footer stays on one line", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 460 });
  await mockApi(page, [
    { pattern: /\/v1\/overview$/, body: { state: "empty", stats: {} } },
    { pattern: /\/v1\/auth\/me$/, body: { username: "admin" } },
  ]);
  await page.goto("/");
  const footer = page.getByText(/Then connect an alert channel/);
  await footer.waitFor();
  const box = (await footer.boundingBox())!;
  expect(box.height).toBeLessThan(24);
});
