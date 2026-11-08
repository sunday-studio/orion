/*
 * Sidebar display state (open, or collapsed to an icon rail). Persisted in local storage.
 * No imports, so it can be unit-tested with `node --test`.
 */

export type SidebarMode = "open" | "collapsed";

export const SIDEBAR_STORAGE_KEY = "orion.console.sidebar";

/** Below this width the sidebar starts collapsed when nothing is stored yet. */
export const NARROW_VIEWPORT = 768;

const MODES: readonly SidebarMode[] = ["open", "collapsed"];

export const isSidebarMode = (value: unknown): value is SidebarMode =>
  typeof value === "string" && (MODES as readonly string[]).includes(value);

/** Reads a stored value; anything unknown falls back to the default for the viewport. */
export const parseSidebarMode = (raw: string | null, viewportWidth: number): SidebarMode => {
  if (isSidebarMode(raw)) return raw;
  // An earlier version also had a fully hidden state; the icon rail replaces it.
  if (raw === "closed") return "collapsed";
  return viewportWidth < NARROW_VIEWPORT ? "collapsed" : "open";
};

type StorageLike = Pick<Storage, "getItem" | "setItem">;

const defaultStorage = (): StorageLike | null => {
  try {
    return typeof localStorage === "undefined" ? null : localStorage;
  } catch {
    return null;
  }
};

export const readSidebarMode = (
  viewportWidth: number,
  storage: StorageLike | null = defaultStorage(),
): SidebarMode => {
  try {
    return parseSidebarMode(storage?.getItem(SIDEBAR_STORAGE_KEY) ?? null, viewportWidth);
  } catch {
    return parseSidebarMode(null, viewportWidth);
  }
};

export const writeSidebarMode = (
  mode: SidebarMode,
  storage: StorageLike | null = defaultStorage(),
): void => {
  try {
    storage?.setItem(SIDEBAR_STORAGE_KEY, mode);
  } catch {
    // The choice still applies until the page reloads.
  }
};
