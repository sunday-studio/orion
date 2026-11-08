import { useCallback, useEffect, useRef, useState } from "react";
import { TOGGLE_SIDEBAR_EVENT } from "@/components/shared/global-shortcut-events";
import { type SidebarMode, readSidebarMode, writeSidebarMode } from "./sidebar-state";

/** Sidebar mode kept in local storage. The toggle event (Cmd/Ctrl B, account menu) hides or shows it. */
export const useSidebarMode = () => {
  const [mode, setModeState] = useState<SidebarMode>(() => readSidebarMode(window.innerWidth));
  const current = useRef(mode);

  const setMode = useCallback((next: SidebarMode) => {
    current.current = next;
    setModeState(next);
    writeSidebarMode(next);
  }, []);

  useEffect(() => {
    const toggle = () => setMode(current.current === "closed" ? "open" : "closed");
    window.addEventListener(TOGGLE_SIDEBAR_EVENT, toggle);
    return () => window.removeEventListener(TOGGLE_SIDEBAR_EVENT, toggle);
  }, [setMode]);

  return { mode, setMode };
};
