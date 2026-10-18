// ⌘K docs search over /search.json, built at compile time from the docs collection.
type Entry = { title: string; description: string; group: string; url: string; headings: { text: string; slug: string }[] };
type Hit = { href: string; title: string; sub: string; kind: "page" | "section"; score: number };

const esc = (s: string) => s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]!);
const icon = {
  page: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20V3H6.5A2.5 2.5 0 0 0 4 5.5zM4 19.5V22h16v-2.5"/></svg>',
  section: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"><path d="M5 9h14M5 15h14M10 3L8 21M16 3l-2 18"/></svg>',
};

export function initSearch() {
  const dialog = document.querySelector<HTMLElement>("[data-search-dialog]");
  const overlay = document.querySelector<HTMLElement>("[data-search-overlay]");
  const input = document.querySelector<HTMLInputElement>("[data-search-input]");
  const list = document.querySelector<HTMLElement>("[data-search-results]");
  const count = document.querySelector<HTMLElement>("[data-search-count]");
  if (!dialog || !overlay || !input || !list || !count) return;

  let index: Entry[] | null = null;
  let hits: Hit[] = [];
  let sel = 0;
  let last: Element | null = null;

  const load = async () => {
    if (index) return index;
    try { index = (await (await fetch("/search.json")).json()) as Entry[]; } catch { index = []; }
    return index;
  };

  const run = (q: string) => {
    const terms = q.toLowerCase().split(/\s+/).filter(Boolean);
    const out: Hit[] = [];
    for (const e of index ?? []) {
      const title = e.title.toLowerCase();
      const desc = e.description.toLowerCase();
      if (!terms.length) { out.push({ href: e.url, title: e.title, sub: `${e.group} · ${e.description}`, kind: "page", score: 0 }); continue; }
      let s = 0;
      for (const t of terms) s += (title.includes(t) ? 5 : 0) + (desc.includes(t) ? 1 : 0);
      if (s) out.push({ href: e.url, title: e.title, sub: `${e.group} · ${e.description}`, kind: "page", score: s + 1 });
      for (const h of e.headings) {
        const ht = h.text.toLowerCase();
        const hs = terms.reduce((a, t) => a + (ht.includes(t) ? 3 : 0), 0);
        if (hs) out.push({ href: `${e.url}#${h.slug}`, title: `${e.title} › ${h.text}`, sub: e.description, kind: "section", score: hs });
      }
    }
    hits = out.sort((a, b) => b.score - a.score).slice(0, 12);
    sel = 0;
    render(terms);
  };

  const mark = (text: string, terms: string[]) => {
    let html = esc(text);
    for (const t of terms) html = html.replace(new RegExp(`(${t.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")})`, "ig"), "<mark>$1</mark>");
    return html;
  };

  const render = (terms: string[]) => {
    if (!hits.length) { list.innerHTML = '<div class="empty">No results. Try a different word.</div>'; count.textContent = ""; return; }
    const groups: [string, Hit[]][] = [["Pages", hits.filter((h) => h.kind === "page")], ["On this page", hits.filter((h) => h.kind === "section")]];
    let n = 0;
    list.innerHTML = groups.filter(([, g]) => g.length).map(([name, g]) =>
      `<div class="sg">${name}</div>` + g.map((h) => {
        const i = n++;
        return `<a class="ri${i === sel ? " sel" : ""}" href="${esc(h.href)}" data-i="${i}"><span class="ico">${icon[h.kind]}</span><div><b>${mark(h.title, terms)}</b><span class="d">${esc(h.sub)}</span></div><span class="en">↵</span></a>`;
      }).join("")).join("");
    count.textContent = `${hits.length} result${hits.length === 1 ? "" : "s"}`;
    hits = groups.flatMap(([, g]) => g);
  };

  const open = async () => {
    last = document.activeElement;
    overlay.hidden = false; dialog.hidden = false;
    document.body.style.overflow = "hidden";
    input.value = ""; input.focus();
    await load();
    run("");
  };
  const close = () => {
    overlay.hidden = true; dialog.hidden = true;
    document.body.style.overflow = "";
    (last as HTMLElement | null)?.focus?.();
  };
  const move = (d: number) => {
    if (!hits.length) return;
    sel = (sel + d + hits.length) % hits.length;
    list.querySelectorAll(".ri").forEach((el, i) => el.classList.toggle("sel", i === sel));
    list.querySelector(".ri.sel")?.scrollIntoView({ block: "nearest" });
  };

  document.querySelectorAll("[data-search-open]").forEach((b) => b.addEventListener("click", open));
  overlay.addEventListener("click", close);
  input.addEventListener("input", () => run(input.value));
  input.addEventListener("keydown", (e) => {
    if (e.key === "ArrowDown") { e.preventDefault(); move(1); }
    else if (e.key === "ArrowUp") { e.preventDefault(); move(-1); }
    else if (e.key === "Enter") { const h = hits[sel]; if (h) location.href = h.href; }
  });
  document.addEventListener("keydown", (e) => {
    const typing = /^(input|textarea|select)$/i.test((e.target as HTMLElement)?.tagName ?? "");
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") { e.preventDefault(); dialog.hidden ? open() : close(); }
    else if (e.key === "/" && !typing && dialog.hidden) { e.preventDefault(); open(); }
    else if (e.key === "Escape" && !dialog.hidden) close();
  });
}
