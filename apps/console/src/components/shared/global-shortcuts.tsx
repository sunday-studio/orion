import { Suspense, lazy, useEffect, useState } from "react";
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

export const GlobalShortcuts = () => {
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
      if (event.key === "?" && !isTyping(event.target)) {
        event.preventDefault();
        setShortcutsOpen(true);
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
  }, [paletteOpen, shortcutsOpen]);

  return (
    <Suspense fallback={null}>
      {paletteOpen && <CommandPalette open onOpenChange={setPaletteOpen} />}
      {shortcutsOpen && <ShortcutsSheet open onOpenChange={setShortcutsOpen} />}
    </Suspense>
  );
};
