/*
 * Navigation model for the sidebar (D-026 and D-027 in docs/plans/decision-log.md).
 * Pure data and functions with no imports, so the active-page and open-group rules
 * can be unit-tested with `node --test`.
 */

export type NavIconName = "overview" | "incidents" | "monitoring" | "logs" | "notify" | "settings";

export type NavPage = {
  id: string;
  label: string;
  /** Where the link goes, including a query string when the page is a `?tab=` view. */
  to: string;
};

export type NavLeaf = NavPage & { icon: NavIconName };

export type NavGroup = {
  id: string;
  label: string;
  icon: NavIconName;
  pages: readonly NavPage[];
};

export const NAV_LEAVES: readonly NavLeaf[] = [
  { id: "overview", label: "Overview", to: "/", icon: "overview" },
  { id: "incidents", label: "Incidents", to: "/incidents", icon: "incidents" },
];

export const NAV_GROUPS: readonly NavGroup[] = [
  {
    id: "monitoring",
    label: "Monitoring",
    icon: "monitoring",
    pages: [
      { id: "services", label: "Services", to: "/services" },
      { id: "monitors", label: "Monitors", to: "/monitors" },
      { id: "servers", label: "Servers", to: "/servers" },
    ],
  },
  {
    id: "logs",
    label: "Logs",
    icon: "logs",
    pages: [
      { id: "app-logs", label: "App logs", to: "/logs" },
      { id: "events", label: "Events", to: "/logs?tab=status" },
      { id: "core-logs", label: "Core logs", to: "/logs?tab=internal" },
      { id: "apps", label: "Apps", to: "/logs/apps" },
    ],
  },
  {
    id: "notify",
    label: "Notify",
    icon: "notify",
    pages: [
      { id: "notifications", label: "Notifications", to: "/alerts" },
      { id: "channels", label: "Channels", to: "/alerts/channels" },
      { id: "rules", label: "Rules", to: "/alerts/rules" },
      { id: "routes", label: "Routes", to: "/alerts/routes" },
      { id: "status-pages", label: "Status pages", to: "/status-pages" },
    ],
  },
  {
    id: "settings",
    label: "Settings",
    icon: "settings",
    pages: [
      { id: "general", label: "General", to: "/settings/general" },
      { id: "lifecycle", label: "Data lifecycle", to: "/settings/lifecycle" },
      { id: "security", label: "Security", to: "/settings/security" },
      { id: "system", label: "System", to: "/settings/system" },
      { id: "audit", label: "Audit log", to: "/settings/audit" },
    ],
  },
];

/** Where the current URL sits in the sidebar. Both ids are null on pages the sidebar does not list. */
export type ActiveNav = { groupId: string | null; pageId: string | null };

const NONE: ActiveNav = { groupId: null, pageId: null };

const leaf = (pageId: string): ActiveNav => ({ groupId: null, pageId });
const inGroup = (groupId: string, pageId: string): ActiveNav => ({ groupId, pageId });

/** True when `pathname` is `base` or a child path of it (`/logs` matches `/logs/apps`, not `/logsx`). */
const within = (pathname: string, base: string) =>
  pathname === base || pathname.startsWith(`${base}/`);

const SETTINGS_PAGES = new Set(["general", "lifecycle", "security", "system", "audit"]);

const logsPage = (search: string): string => {
  const tab = new URLSearchParams(search).get("tab");
  if (tab === "status") return "events";
  if (tab === "internal") return "core-logs";
  return "app-logs";
};

/** Maps a location to the sidebar page it belongs to. Detail pages light up their list page. */
export const resolveActiveNav = (rawPathname: string, search = ""): ActiveNav => {
  const pathname = rawPathname.length > 1 ? rawPathname.replace(/\/+$/, "") : rawPathname;
  if (pathname === "/" || within(pathname, "/health-issues")) return leaf("overview");
  if (within(pathname, "/incidents")) return leaf("incidents");

  if (within(pathname, "/services")) return inGroup("monitoring", "services");
  if (within(pathname, "/monitors")) return inGroup("monitoring", "monitors");
  if (within(pathname, "/servers")) return inGroup("monitoring", "servers");

  if (within(pathname, "/logs/apps")) return inGroup("logs", "apps");
  if (within(pathname, "/logs")) return inGroup("logs", logsPage(search));

  if (within(pathname, "/alerts/channels")) return inGroup("notify", "channels");
  if (within(pathname, "/alerts/rules")) return inGroup("notify", "rules");
  if (within(pathname, "/alerts/routes")) return inGroup("notify", "routes");
  if (within(pathname, "/alerts")) return inGroup("notify", "notifications");
  if (within(pathname, "/status-pages")) return inGroup("notify", "status-pages");

  if (within(pathname, "/settings")) {
    const section = pathname.split("/")[2] ?? "";
    return SETTINGS_PAGES.has(section)
      ? inGroup("settings", section)
      : inGroup("settings", "general");
  }
  return NONE;
};

/** Per-group choices the user made by hand: true forces open, false forces closed. */
export type GroupOverrides = Readonly<Record<string, boolean>>;

/** The group holding the current page is open; the rest are closed unless the user opened them. */
export const isGroupOpen = (
  groupId: string,
  activeGroupId: string | null,
  overrides: GroupOverrides,
): boolean => overrides[groupId] ?? groupId === activeGroupId;

/** Flips one group. Choices made on other groups are kept. */
export const toggleGroup = (
  groupId: string,
  activeGroupId: string | null,
  overrides: GroupOverrides,
): GroupOverrides => ({
  ...overrides,
  [groupId]: !isGroupOpen(groupId, activeGroupId, overrides),
});
