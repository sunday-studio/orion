// Renders the Open Graph share images with headless Chrome. No dependencies.
//
//   node scripts/og/generate.mjs
//
// Env: CHROME            path to a Chrome or Chromium binary (defaults to the macOS install)
//      ORION_OG_FONT_CSS path to a stylesheet that defines the licensed "TWK Lausanne" font (optional)
//
// Sizes: 1200x630 (Facebook, LinkedIn, Slack, Discord), 1200x600 (X / Twitter) and 1200x1200 (square cards).
// Layouts: home, docs home, and one per docs article (title-led).
import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, dirname } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..", "..");
const chrome = process.env.CHROME ?? "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const fontCss = process.env.ORION_OG_FONT_CSS ? pathToFileURL(process.env.ORION_OG_FONT_CSS).href : "";
const SIZES = [[1200, 630], [1200, 600], [1200, 1200]];

// ---- pages -------------------------------------------------------------------------------------
const pages = [
  { out: "og/home", variant: "stage", kicker: "Self-hosted monitoring", title: "Run monitoring and incident response on <em>a machine you own.</em>", sub: "Uptime monitoring, incident response and logs for personal projects and small teams." },
  { out: "og/docs-home", variant: "stage", kicker: "Documentation", title: "Run Orion <em>yourself.</em>", sub: "Install it, connect your servers and apps, and keep it running." },
];
const docsDir = join(root, "src/content/docs");
for (const file of readdirSync(docsDir).filter((f) => /\.mdx?$/.test(f))) {
  const fm = readFileSync(join(docsDir, file), "utf8").split("---")[1] ?? "";
  const get = (k) => (fm.match(new RegExp(`^${k}:\\s*"?(.*?)"?\\s*$`, "m")) ?? [])[1] ?? "";
  pages.push({ out: `og/docs/${file.replace(/\.mdx?$/, "")}`, variant: "article", kicker: get("group"), title: esc(get("title")), sub: esc(get("description")) });
}
function esc(s) { return s.replace(/&/g, "&amp;").replace(/</g, "&lt;"); }

// ---- drawing -----------------------------------------------------------------------------------
function rng(seed) { return () => { seed |= 0; seed = (seed + 0x6d2b79f5) | 0; let t = Math.imul(seed ^ (seed >>> 15), 1 | seed); t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t; return ((t ^ (t >>> 14)) >>> 0) / 4294967296; }; }
function dots(w, h, n) {
  const r = rng(11); let out = "";
  for (let i = 0; i < n; i++) out += `<circle cx="${Math.round(r() * w)}" cy="${Math.round(r() * h)}" r="${[0.9, 1.2, 1.6][Math.floor(r() * 3)]}" fill="#6ee7b7" opacity="${[0.18, 0.28, 0.4][Math.floor(r() * 3)]}"/>`;
  return out;
}
function ecg(y, s, alarm) {
  const beat = (x, k = 1) => `L${x} ${y} L${x + 14} ${y} L${x + 22} ${y - 14 * k} L${x + 30} ${y} L${x + 44} ${y} L${x + 52} ${y + 10 * k} L${x + 62} ${y - 70 * k} L${x + 72} ${y + 34 * k} L${x + 82} ${y} L${x + 104} ${y} L${x + 118} ${y - 12 * k} L${x + 134} ${y}`;
  const xs = alarm ? [40, 230, 420, 610] : [40, 230, 420, 610, 800];
  return { d: `M0 ${y} ` + xs.map((x) => beat(x)).join("") + (alarm ? beat(800, s) : "") + ` L1000 ${y} L1200 ${y}`, px: 862, py: y - 70 * s };
}
const logo = '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><circle cx="12" cy="12" r="3"/><circle cx="12" cy="12" r="9" stroke-dasharray="2 4"/></svg>';

function html(p, w, h) {
  const square = h > 800, short = h < 620;
  const alarm = p.variant === "stage";
  const baseY = h - (square ? 130 : 105) + (short ? 0 : 0);
  const line = ecg(baseY, square ? 2.4 : 1.5, alarm);
  const top = p.variant === "stage" ? (square ? 250 : short ? 130 : 148) : (square ? 300 : short ? 150 : 170);
  const h1 = p.variant === "stage" ? (square ? 104 : 68) : (square ? 100 : 76);
  const sub = square ? 34 : 25;
  const grad = alarm
    ? '<stop offset="0" stop-color="#6ee7b7" stop-opacity="0"/><stop offset=".15" stop-color="#6ee7b7" stop-opacity=".9"/><stop offset=".66" stop-color="#6ee7b7"/><stop offset=".72" stop-color="#fb7185"/><stop offset="1" stop-color="#fb7185" stop-opacity="0"/>'
    : '<stop offset="0" stop-color="#6ee7b7" stop-opacity="0"/><stop offset=".15" stop-color="#6ee7b7" stop-opacity=".7"/><stop offset=".85" stop-color="#6ee7b7" stop-opacity=".7"/><stop offset="1" stop-color="#6ee7b7" stop-opacity="0"/>';
  const alarmDot = alarm ? `<circle cx="${line.px}" cy="${line.py}" r="6" fill="#fb7185"/><circle cx="${line.px}" cy="${line.py}" r="15" fill="none" stroke="#fb7185" stroke-opacity=".45" stroke-width="2"/>` : "";
  const crumb = p.variant === "article" ? `<div class="crumb">orion / docs / ${p.kicker.toLowerCase()}</div>` : `<div class="crumb">github.com/sunday-studio/orion</div>`;
  const kickerTop = p.variant === "article" ? `<span class="docs">docs</span>` : "";
  return `<!doctype html><html><head><meta charset="utf-8">${fontCss ? `<link rel="stylesheet" href="${fontCss}">` : ""}<style>
html,body{margin:0;background:#142a23}
.o{position:relative;width:${w}px;height:${h}px;overflow:hidden;background:#142a23;color:#fff;font:300 20px/1.4 "TWK Lausanne",system-ui,-apple-system,"Segoe UI",sans-serif;-webkit-font-smoothing:antialiased}
.glow{position:absolute;left:380px;top:${-260}px;width:900px;height:${square ? 1000 : 760}px;background:radial-gradient(closest-side,rgba(110,231,183,.16),rgba(110,231,183,0))}
svg.l{position:absolute;left:0;top:0}
.lg{position:absolute;left:72px;top:${short ? 48 : 56}px;display:flex;align-items:center;gap:12px;font-size:28px;font-weight:500;letter-spacing:-.01em}
.mk{width:40px;height:40px;border-radius:11px;background:#6ee7b7;color:#142a23;display:grid;place-items:center}
.docs{font:400 15px ui-monospace,"IBM Plex Mono",Menlo,monospace;color:#8fb3a3;border:1px solid rgba(255,255,255,.18);border-radius:5px;padding:1px 8px;margin-left:4px}
.k{position:absolute;right:72px;top:${short ? 58 : 66}px;font:500 16px ui-monospace,"IBM Plex Mono",Menlo,monospace;letter-spacing:.1em;text-transform:uppercase;color:#6ee7b7}
.b{position:absolute;left:72px;right:72px;top:${top}px}
h1{margin:0;font-size:${h1}px;line-height:1.04;letter-spacing:-.035em;font-weight:300;max-width:${square ? 1050 : 1060}px}
h1 em{font-style:normal;color:#6ee7b7}
.sub{margin-top:${square ? 36 : 26}px;font-size:${sub}px;line-height:1.35;color:#9fc4b3;max-width:${p.variant === "article" ? 900 : 1060}px;${p.variant === "article" ? "display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden" : ""}}
.crumb{position:absolute;left:72px;bottom:${square ? 52 : 28}px;font:400 ${square ? 20 : 16}px ui-monospace,"IBM Plex Mono",Menlo,monospace;color:#6ea590}
</style></head><body><div class="o">${alarm ? '<div class="glow"></div>' : ""}
<svg class="l" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">${alarm ? dots(w, h, square ? 240 : 120) : ""}
<defs><linearGradient id="g" x1="0" x2="1">${grad}</linearGradient></defs>
<path d="${line.d}" fill="none" stroke="url(#g)" stroke-width="${square ? 4 : 3}" stroke-linejoin="round" stroke-linecap="round"/>${alarmDot}</svg>
<div class="lg"><span class="mk">${logo}</span>Orion${kickerTop}</div><div class="k">${p.kicker}</div>
<div class="b"><h1>${p.title}</h1><div class="sub">${p.sub}</div></div>${crumb}</div></body></html>`;
}

// ---- render ------------------------------------------------------------------------------------
const tmp = mkdtempSync(join(tmpdir(), "orion-og-"));
let count = 0;
for (const p of pages) {
  for (const [w, h] of SIZES) {
    const file = join(root, "public", `${p.out}-${w}x${h}.png`);
    mkdirSync(dirname(file), { recursive: true });
    const src = join(tmp, "page.html");
    writeFileSync(src, html(p, w, h));
    execFileSync(chrome, ["--headless", "--disable-gpu", "--hide-scrollbars", "--force-device-scale-factor=1", "--virtual-time-budget=3000", `--window-size=${w},${h}`, `--screenshot=${file}`, pathToFileURL(src).href], { stdio: "ignore" });
    count++;
  }
}
console.log(`Rendered ${count} images for ${pages.length} pages.`);
