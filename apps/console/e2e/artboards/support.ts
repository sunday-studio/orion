import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { type Page, expect } from "@playwright/test";

type Json = Record<string, unknown>;

/** Core responses are wrapped as { success, data }; mock them the same way. */
export const envelope = (data: unknown) => JSON.stringify({ success: true, data });

export type RouteMock = { pattern: RegExp; body: Json | ((url: URL) => Json) };

export const mockApi = async (
  page: Page,
  mocks: RouteMock[],
  options: { theme?: "light" | "dark" } = {},
) => {
  await page.addInitScript((theme) => {
    if (theme) localStorage.setItem("orion.theme.choice", theme);
    localStorage.setItem("orion_token", "artboard-test-token");
    localStorage.setItem("orion_username", "admin");
  }, options.theme);
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
  if (process.env.REVIEW_OUT) {
    // Let open/close transitions finish so the review image is not captured mid-fade.
    await page.waitForTimeout(350);
    await writeReviewImage(page, name, theme, await page.screenshot());
  }
  await expect(page).toHaveScreenshot(`${name}-${theme}.png`, {
    maxDiffPixelRatio: 0.06,
    animations: "disabled",
    fullPage: false,
  });
};

/**
 * Writes artboard-vs-app side-by-side images for human review when REVIEW_OUT is set,
 * for example REVIEW_OUT=docs/design/review/f2 (relative to the repository root).
 */
const writeReviewImage = async (page: Page, name: string, theme: string, actual: Buffer) => {
  const out = process.env.REVIEW_OUT;
  if (!out) return;
  const repoRoot = resolve(process.cwd(), "../..");
  const artboard = readFileSync(
    resolve(repoRoot, `docs/design/artboards/png/${name}-${theme}.png`),
  ).toString("base64");
  const browser = page.context().browser();
  if (!browser) return;
  const compare = await browser.newPage({ viewport: { width: 2580, height: 400 } });
  await compare.setContent(
    `<body style="margin:0;background:#777;display:flex;gap:20px;align-items:flex-start"><img src="data:image/png;base64,${artboard}"><img src="data:image/png;base64,${actual.toString("base64")}"></body>`,
  );
  await compare.waitForLoadState("load");
  const height = await compare.evaluate(() => document.body.scrollHeight);
  await compare.setViewportSize({ width: 2580, height });
  const target = resolve(repoRoot, out, `${name}-${theme}.png`);
  mkdirSync(dirname(target), { recursive: true });
  writeFileSync(target, await compare.screenshot());
  await compare.close();
};
