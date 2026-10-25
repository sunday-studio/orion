// Console tour: a sliding-thumb segmented control (same behaviour as the app) that autoplays until the visitor picks a tab.
const INTERVAL = 4500;

export function initTour() {
  document.querySelectorAll<HTMLElement>("[data-tour]").forEach((root) => {
    const items = Array.from(root.querySelectorAll<HTMLButtonElement>(".segi"));
    const thumb = root.querySelector<HTMLElement>(".thumb");
    const shots = Array.from(root.querySelectorAll<HTMLElement>(".fi"));
    const caps = Array.from(root.querySelectorAll<HTMLElement>(".cp"));
    if (!thumb || !items.length) return;

    let current = 0;
    let auto = !matchMedia("(prefers-reduced-motion: reduce)").matches;
    let timer: number | undefined;

    const place = () => {
      const a = items[current];
      thumb.style.left = `${a.offsetLeft}px`;
      thumb.style.width = `${a.offsetWidth}px`;
    };
    const restartProgress = () => {
      root.classList.remove("run");
      void root.offsetWidth;
      if (auto) root.classList.add("run");
    };
    const show = (i: number, scroll = false) => {
      current = i;
      items.forEach((b, n) => {
        b.setAttribute("aria-pressed", String(n === i));
        n === i ? b.setAttribute("data-segment-active", "") : b.removeAttribute("data-segment-active");
      });
      shots.forEach((s, n) => s.classList.toggle("is-active", n === i));
      caps.forEach((c, n) => c.classList.toggle("is-active", n === i));
      place();
      if (scroll) items[i].scrollIntoView({ inline: "nearest", block: "nearest" });
      restartProgress();
    };
    const stop = () => { if (timer) window.clearInterval(timer); timer = undefined; };
    const start = () => { stop(); if (auto) timer = window.setInterval(() => show((current + 1) % items.length), INTERVAL); };
    const takeOver = (i: number) => { auto = false; stop(); show(i); };

    items.forEach((b, i) => {
      b.addEventListener("click", () => takeOver(i));
      b.addEventListener("keydown", (e) => {
        if (e.key !== "ArrowRight" && e.key !== "ArrowLeft") return;
        e.preventDefault();
        const next = (i + (e.key === "ArrowRight" ? 1 : -1) + items.length) % items.length;
        takeOver(next);
        items[next].focus();
      });
    });
    const frame = root.querySelector(".frame");
    frame?.addEventListener("mouseenter", () => { if (auto) { stop(); root.classList.remove("run"); } });
    frame?.addEventListener("mouseleave", () => { if (auto) { start(); restartProgress(); } });

    place();
    requestAnimationFrame(() => thumb.classList.add("ready"));
    new ResizeObserver(place).observe(root.querySelector(".seg") as Element);

    if ("IntersectionObserver" in window) {
      new IntersectionObserver((entries) => {
        entries.forEach((e) => { if (e.isIntersecting) { start(); restartProgress(); } else { stop(); root.classList.remove("run"); } });
      }, { threshold: 0.35 }).observe(root);
    } else start();
  });
}
