/*
 * Old Console URLs and where they go now (docs/plans/information-architecture-review.md section 5.4).
 * Returns the new location (path plus query) or null when the URL is current.
 * No imports, so it can be unit-tested with `node --test`.
 */

const SETTINGS_SECTIONS = new Set(["general", "lifecycle", "security", "system", "audit"]);
const ALERT_SECTIONS = new Set(["channels", "rules", "routes"]);

/** The query string without `key`, as `?a=b` or an empty string. */
const withoutParam = (search: string, key: string): string => {
  const params = new URLSearchParams(search);
  params.delete(key);
  const rest = params.toString();
  return rest ? `?${rest}` : "";
};

const trim = (pathname: string) => (pathname.length > 1 ? pathname.replace(/\/+$/, "") : pathname);

export const resolveLegacyRedirect = (rawPathname: string, search = ""): string | null => {
  const pathname = trim(rawPathname);
  const tab = new URLSearchParams(search).get("tab") ?? "";

  if (pathname === "/overview") return "/";

  // Settings: one URL with tabs became one URL per page; Apps moved under Logs.
  if (pathname === "/settings") {
    if (tab === "apps") return `/logs/apps${withoutParam(search, "tab")}`;
    const section = SETTINGS_SECTIONS.has(tab) ? tab : "general";
    return `/settings/${section}${withoutParam(search, "tab")}`;
  }
  if (pathname === "/settings/apps") return `/logs/apps${search}`;
  if (pathname.startsWith("/settings/apps/")) return `/logs/apps/${pathname.slice(15)}${search}`;

  // The connect wizard now belongs to Logs.
  if (pathname === "/apps/new") return `/logs/apps/new${search}`;

  // Alerts: the tab query became a path (`/alerts?tab=rules` is `/alerts/rules`).
  if (pathname === "/alerts" && tab !== "") {
    if (ALERT_SECTIONS.has(tab)) return `/alerts/${tab}${withoutParam(search, "tab")}`;
    if (tab === "logs") return `/alerts${withoutParam(search, "tab")}`;
  }

  // Server tab "Apps" is now "Usage".
  if (/^\/servers\/[^/]+$/.test(pathname) && tab === "apps") {
    const params = new URLSearchParams(search);
    params.set("tab", "usage");
    return `${pathname}?${params.toString()}`;
  }
  return null;
};
