import { expect, test } from "@playwright/test";
import { expectMatchesArtboard } from "./support";

test.use({ viewport: { width: 1280, height: 720 } });

test("Login matches the artboard", async ({ page }) => {
  await page.route("**/v1/auth/login", (route) =>
    route.fulfill({
      status: 401,
      contentType: "application/json",
      body: JSON.stringify({ success: false, error: "invalid credentials" }),
    }),
  );
  await page.goto("/login");
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill("password");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.getByText("Username or password is incorrect.").waitFor();
  await expectMatchesArtboard(page, "login", 720);
});

for (const width of [1280, 1440, 1920, 2560]) {
  test(`Login layout is balanced at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/login");
    const form = page.locator("form");
    await form.waitFor();
    const box = (await form.boundingBox())!;
    // The form is centered in the right half with a fixed width, not pinned to the edge.
    expect(box.width).toBeLessThanOrEqual(340);
    const centre = box.x + box.width / 2;
    expect(Math.abs(centre - width * 0.75)).toBeLessThan(width * 0.02);
    // The hero text sits in the left half and never crosses the centre line.
    const headline = (await page.getByRole("img", { name: /KNOW BEFORE/i }).boundingBox())!;
    expect(headline.x + headline.width).toBeLessThanOrEqual(width / 2);
    if (process.env.ARTBOARD_SHOTS) {
      await page.screenshot({ path: `${process.env.ARTBOARD_SHOTS}/login-${width}.png` });
    }
  });
}

test("Login explains when Core cannot be reached", async ({ page }) => {
  await page.route("**/v1/auth/login", (route) => route.abort("connectionrefused"));
  await page.goto("/login");
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill("password");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByText(/Can't reach Core at .*\. Is it running\?/)).toBeVisible();
});
