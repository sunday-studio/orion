// Usage: node e2e/support/side-by-side.mjs <artboard.png> <actual.png> <out.png>
// Renders the artboard (left) and the running app (right) in one image for quick visual review.
import { readFileSync } from "node:fs";
import { chromium } from "@playwright/test";

const [artboard, actual, out] = process.argv.slice(2);
const uri = (path) => `data:image/png;base64,${readFileSync(path).toString("base64")}`;
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 2580, height: 400 } });
await page.setContent(
  `<body style="margin:0;background:#888;display:flex;gap:20px"><img src="${uri(artboard)}"><img src="${uri(actual)}"></body>`,
);
await page.waitForLoadState("load");
const height = await page.evaluate(() => document.body.scrollHeight);
await page.setViewportSize({ width: 2580, height });
await page.screenshot({ path: out });
await browser.close();
