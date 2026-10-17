export type ThemePreference = "light" | "dark" | "system";

const STORAGE_KEY = "orion.theme";

const resolve = (preference: ThemePreference): "light" | "dark" =>
  preference === "system"
    ? window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light"
    : preference;

export const readThemePreference = (): ThemePreference => {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    return stored === "light" || stored === "dark" || stored === "system" ? stored : "light";
  } catch {
    return "light";
  }
};

/** Applies the theme to <html data-theme> and remembers it per browser. */
export const applyTheme = (preference: ThemePreference) => {
  document.documentElement.dataset.theme = resolve(preference);
  try {
    localStorage.setItem(STORAGE_KEY, preference);
  } catch {
    // The theme still applies for this page view.
  }
};
