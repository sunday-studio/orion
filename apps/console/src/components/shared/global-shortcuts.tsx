import { Suspense, lazy, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  OPEN_PALETTE_EVENT,
  OPEN_SHORTCUTS_EVENT,
} from "@/components/shared/global-shortcut-events";
const CommandPalette = lazy(() =>
  import("@/components/shared/command-palette").then((m) => ({ default: m.CommandPalette })),
);
const ShortcutsSheet = lazy(() =>
  import("@/components/shared/shortcuts-sheet").then((m) => ({ default: m.ShortcutsSheet })),
);

const isTyping = (target: EventTarget | null) => {
  const element = target as HTMLElement | null;
  if (!element) return false;
  return (
    element.isContentEditable ||
    element.tagName === "INPUT" ||
    element.tagName === "TEXTAREA" ||
    element.tagName === "SELECT"
  );
};

const GO_TARGETS: Record<string, string> = {
  o: "/",
  i: "/incidents",
  m: "/monitors",
  s: "/servers",
  a: "/alerts",
  l: "/logs",
};

const focusRow = (direction: 1 | -1) => {
  const rows = Array.from(document.querySelectorAll<HTMLElement>("tbody tr[tabindex]"));
  if (rows.length === 0) return;
  const current = rows.indexOf(document.activeElement as HTMLElement);
  const next = current === -1 ? (direction === 1 ? 0 : rows.length - 1) : current + direction;
  rows[Math.min(Math.max(next, 0), rows.length - 1)]?.focus();
};

export const GlobalShortcuts = () => {
  const navigate = useNavigate();
  const pendingGo = useRef(0);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [shortcutsOpen, setShortcutsOpen] = useState(false);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (!paletteOpen && !shortcutsOpen && document.querySelector('[role="dialog"]')) return;
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setPaletteOpen((open) => !open);
        return;
      }
      if (event.metaKey || event.ctrlKey || event.altKey || isTyping(event.target)) return;
      const key = event.key.toLowerCase();
      if (event.key === "?") {
        event.preventDefault();
        setShortcutsOpen(true);
      } else if (pendingGo.current > Date.now() && GO_TARGETS[key]) {
        event.preventDefault();
        pendingGo.current = 0;
        navigate(GO_TARGETS[key]);
      } else if (key === "g") {
        pendingGo.current = Date.now() + 1000;
      } else if (key === "n") {
        navigate("/monitors?new=1");
      } else if (key === "s") {
        navigate("/servers/new");
      } else if (key === "j" || key === "k") {
        focusRow(key === "j" ? 1 : -1);
      } else if (key === "/") {
        const filter = document.querySelector<HTMLInputElement>('[role="search"] input');
        if (filter) {
          event.preventDefault();
          filter.focus();
        }
      }
    };
    const openPalette = () => setPaletteOpen(true);
    const openShortcuts = () => setShortcutsOpen(true);
    window.addEventListener("keydown", onKeyDown);
    window.addEventListener(OPEN_PALETTE_EVENT, openPalette);
    window.addEventListener(OPEN_SHORTCUTS_EVENT, openShortcuts);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
      window.removeEventListener(OPEN_PALETTE_EVENT, openPalette);
      window.removeEventListener(OPEN_SHORTCUTS_EVENT, openShortcuts);
    };
  }, [navigate, paletteOpen, shortcutsOpen]);

  return (
    <Suspense fallback={null}>
      {paletteOpen && <CommandPalette open onOpenChange={setPaletteOpen} />}
      {shortcutsOpen && <ShortcutsSheet open onOpenChange={setShortcutsOpen} />}
    </Suspense>
  );
};
