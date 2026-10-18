export type ThemePreference = "light" | "dark" | "system";

/** Manual choice only. The older "orion.theme" key stored every visit, so it is never read. */
const STORAGE_KEY = "orion.theme.choice";
const LEGACY_STORAGE_KEY = "orion.theme";
const DARK_QUERY = "(prefers-color-scheme: dark)";

const resolve = (preference: ThemePreference): "light" | "dark" =>
  preference === "system" ? (window.matchMedia(DARK_QUERY).matches ? "dark" : "light") : preference;

/** Follows the device theme unless the user chose Light or Dark in the account menu. */
export const readThemePreference = (): ThemePreference => {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    return stored === "light" || stored === "dark" || stored === "system" ? stored : "system";
  } catch {
    return "system";
  }
};

const paint = (preference: ThemePreference) => {
  document.documentElement.dataset.theme = resolve(preference);
};

/** Applies the theme to <html data-theme> and remembers an explicit choice per browser. */
export const applyTheme = (preference: ThemePreference) => {
  paint(preference);
  try {
    if (preference === "system") localStorage.removeItem(STORAGE_KEY);
    else localStorage.setItem(STORAGE_KEY, preference);
  } catch {
    // The theme still applies for this page view.
  }
};

/**
 * Applies the saved preference at startup and keeps "system" in sync when the OS theme changes.
 * Returns a cleanup function.
 */
export const initTheme = () => {
  try {
    localStorage.removeItem(LEGACY_STORAGE_KEY);
  } catch {
    // Nothing to clean up when storage is unavailable.
  }
  paint(readThemePreference());
  const query = window.matchMedia(DARK_QUERY);
  const onChange = () => {
    if (readThemePreference() === "system") paint("system");
  };
  query.addEventListener("change", onChange);
  return () => query.removeEventListener("change", onChange);
};
