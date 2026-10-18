// Small enhancements. Everything works without JavaScript except search, copy and saved progress.
import { initSearch } from "./search";
import { initTour } from "./tour";

function copyText(text: string, done: () => void) {
  navigator.clipboard?.writeText(text).then(done, done);
}

function enhanceCodeBlocks() {
  document.querySelectorAll<HTMLPreElement>(".art pre.astro-code").forEach((pre) => {
    if (pre.parentElement?.classList.contains("code-wrap") || pre.closest(".panes")) return;
    const wrap = document.createElement("div");
    wrap.className = "code-wrap";
    pre.parentNode?.insertBefore(wrap, pre);
    wrap.appendChild(pre);
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "copy-btn";
    btn.textContent = "Copy";
    btn.addEventListener("click", () => {
      copyText(pre.innerText.replace(/\n$/, ""), () => {
        btn.textContent = "Copied";
        setTimeout(() => (btn.textContent = "Copy"), 1600);
      });
    });
    wrap.appendChild(btn);
  });
}

function enhanceTabCopy() {
  document.querySelectorAll<HTMLElement>(".code[data-tabs] [data-copy]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const root = btn.closest<HTMLElement>(".code");
      const checked = root?.querySelector<HTMLInputElement>("input[type=radio]:checked");
      const index = checked ? Number(checked.dataset.index) : 0;
      const pre = root?.querySelectorAll<HTMLElement>(".panes pre")[index];
      if (!pre) return;
      copyText(pre.innerText.replace(/\n$/, ""), () => {
        btn.dataset.state = "copied";
        setTimeout(() => delete btn.dataset.state, 1600);
      });
    });
  });
}

function enhanceGateCopy() {
  document.querySelectorAll<HTMLElement>(".gp").forEach((panel) => {
    const label = panel.querySelector<HTMLLabelElement>(".copy");
    const pre = panel.querySelector("pre");
    if (!label || !pre) return;
    label.addEventListener("click", () => {
      const lines = (pre.textContent ?? "").split("\n");
      const cmds: string[] = [];
      for (const line of lines) {
        if (/^[✔!]/.test(line) || /^open /.test(line)) break;
        cmds.push(line.replace(/^\$ /, ""));
      }
      copyText(cmds.join("\n"), () => {});
    });
  });
}

function tocSpy() {
  const links = Array.from(document.querySelectorAll<HTMLAnchorElement>(".toc a[href^='#']"));
  if (!links.length || !("IntersectionObserver" in window)) return;
  const byId = new Map(links.map((a) => [a.getAttribute("href")!.slice(1), a]));
  const io = new IntersectionObserver(
    (entries) => {
      for (const e of entries) {
        if (!e.isIntersecting) continue;
        links.forEach((a) => a.classList.remove("on"));
        byId.get(e.target.id)?.classList.add("on");
      }
    },
    { rootMargin: "-80px 0px -70% 0px" },
  );
  byId.forEach((_, id) => {
    const el = document.getElementById(id);
    if (el) io.observe(el);
  });
}

function savedProgress() {
  const key = "orion-docs-progress";
  let saved: Record<string, boolean> = {};
  try { saved = JSON.parse(localStorage.getItem(key) ?? "{}"); } catch { /* private mode */ }
  document.querySelectorAll<HTMLInputElement>("input.sx").forEach((box) => {
    if (saved[box.id]) box.checked = true;
    box.addEventListener("change", () => {
      saved[box.id] = box.checked;
      try { localStorage.setItem(key, JSON.stringify(saved)); } catch { /* ignore */ }
    });
  });
}

function storyboard() {
  const grid = document.querySelector<HTMLElement>("[data-storyboard]");
  if (!grid) return;
  if (!("IntersectionObserver" in window)) { grid.classList.add("go"); return; }
  const io = new IntersectionObserver((entries) => {
    if (entries.some((e) => e.isIntersecting)) { grid.classList.add("go"); io.disconnect(); }
  }, { threshold: 0.4 });
  io.observe(grid);
}

function sideDrawer() {
  const side = document.getElementById("docs-side");
  const btn = document.querySelector<HTMLButtonElement>("[data-side-toggle]");
  const overlay = document.querySelector<HTMLElement>("[data-side-overlay]");
  if (!side || !btn || !overlay) return;
  const set = (open: boolean) => {
    side.classList.toggle("open", open);
    overlay.hidden = !open;
    btn.setAttribute("aria-expanded", String(open));
    document.body.style.overflow = open ? "hidden" : "";
  };
  btn.addEventListener("click", () => set(!side.classList.contains("open")));
  overlay.addEventListener("click", () => set(false));
  side.addEventListener("click", (e) => { if ((e.target as HTMLElement).closest("a")) set(false); });
  document.addEventListener("keydown", (e) => { if (e.key === "Escape") set(false); });
}

enhanceCodeBlocks();
enhanceTabCopy();
enhanceGateCopy();
tocSpy();
savedProgress();
initSearch();
sideDrawer();
initTour();
storyboard();
