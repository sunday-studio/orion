import { type Page, expect } from "@playwright/test";

type Json = Record<string, unknown>;

/** Core responses are wrapped as { success, data }; mock them the same way. */
export const envelope = (data: unknown) => JSON.stringify({ success: true, data });

export type RouteMock = { pattern: RegExp; body: Json | ((url: URL) => Json) };

export const mockApi = async (page: Page, mocks: RouteMock[]) => {
  await page.addInitScript(() => {
    localStorage.setItem("orion_token", "artboard-test-token");
    localStorage.setItem("orion_username", "admin");
  });
  await page.route("**/v1/**", async (route) => {
    const url = new URL(route.request().url());
    const match = mocks.find((mock) => mock.pattern.test(url.pathname));
    if (!match) {
      await route.fulfill({ status: 200, contentType: "application/json", body: envelope({}) });
      return;
    }
    const body = typeof match.body === "function" ? match.body(url) : match.body;
    await route.fulfill({ status: 200, contentType: "application/json", body: envelope(body) });
  });
};

/** Freeze the clock so relative times ("Open for 30m 12s") match the artboard. */
export const freezeClock = async (page: Page, iso: string) => {
  await page.clock.install({ time: new Date(iso) });
};

/**
 * Compare the page with the exported artboard PNG (docs/design/artboards/png/<name>-<theme>.png).
 * Fonts differ (the artboards use licensed TWK Lausanne), so the tolerance is deliberately loose;
 * the diff image written on failure shows layout drift.
 */
export const expectMatchesArtboard = async (
  page: Page,
  name: string,
  height: number,
  theme: "light" | "dark" = "light",
) => {
  await page.setViewportSize({ width: 1280, height });
  await page.addStyleTag({ content: "*{caret-color:transparent!important}" });
  if (process.env.ARTBOARD_SHOTS) {
    await page.screenshot({ path: `${process.env.ARTBOARD_SHOTS}/${name}-${theme}.png` });
  }
  await expect(page).toHaveScreenshot(`${name}-${theme}.png`, {
    maxDiffPixelRatio: 0.06,
    animations: "disabled",
    fullPage: false,
  });
};
