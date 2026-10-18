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

const userCounts = (monitors: number, servers: number) => [
  {
    pattern: /\/v1\/overview$/,
    body: {
      state: monitors + servers === 0 ? "empty" : "ok",
      ongoing_incident_count: 0,
      health_issue_count: 0,
      ongoing_incidents: [],
      recent_incidents: [],
      stats: {
        monitors_total: monitors,
        monitors_up: monitors,
        servers_total: servers,
        servers_reporting: servers,
      },
    },
  },
  { pattern: /\/v1\/auth\/me$/, body: { username: "admin" } },
  { pattern: /\/v1\/health\/issues$/, body: { count: 0, issues: [] } },
];

test("a fresh database shows first run, even if a system item exists", async ({ page }) => {
  // Overview says "ok" but there are zero user monitors and servers (only Core's own worker).
  await mockApi(page, [
    {
      pattern: /\/v1\/overview$/,
      body: {
        state: "ok",
        stats: { monitors_total: 0, monitors_up: 1, servers_total: 0, servers_reporting: 1 },
      },
    },
    { pattern: /\/v1\/auth\/me$/, body: { username: "admin" } },
    { pattern: /\/v1\/health\/issues$/, body: { count: 0, issues: [] } },
  ]);
  await page.goto("/");
  await expect(page.getByText("Nothing is monitored yet")).toBeVisible();
});

test("counts are pluralised correctly", async ({ page }) => {
  await mockApi(page, userCounts(1, 1));
  await page.goto("/");
  await expect(page.getByText("1 monitor passing, 1 server reporting")).toBeVisible();
  await mockApi(page, userCounts(2, 0));
  await page.reload();
  await expect(page.getByText("2 monitors passing, 0 servers reporting")).toBeVisible();
});

test("the first-run footer wraps instead of overflowing on small screens", async ({ page }) => {
  await mockApi(page, userCounts(0, 0));
  for (const width of [1024, 375]) {
    await page.setViewportSize({ width, height: 700 });
    await page.goto("/");
    await page.getByText("Nothing is monitored yet").waitFor();
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(overflow).toBeLessThanOrEqual(0);
  }
});
