import { test } from "@playwright/test";
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
