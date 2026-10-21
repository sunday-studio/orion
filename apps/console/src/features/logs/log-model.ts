import type { ApiLogEntry, ApiStatusLogEntry } from "@/orion-sdk";

/** One line of any log screen, normalised so the table and drawer render all three kinds. */
export type LogRow = {
  id: string;
  at?: string;
  level: string;
  /** Source app, internal component or server, depending on the screen. */
  label?: string;
  monitor?: string;
  title: string;
  detail?: string;
  inferred?: boolean;
  /** Plain-text app: no event name and no attributes. */
  plain?: boolean;
};

const flatten = (value: unknown, prefix = "", out: [string, string][] = []) => {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    for (const [key, item] of Object.entries(value as Record<string, unknown>))
      flatten(item, prefix ? `${prefix}.${key}` : key, out);
  } else if (prefix) {
    out.push([prefix, String(value)]);
  }
  return out;
};

const attributeSummary = (attributes: unknown, limit = 4) =>
  flatten(attributes)
    .slice(0, limit)
    .map(([key, value]) => `${key}=${value}`)
    .join(" ");

export const levelLabel = (level?: string) =>
  ({ warn: "Warn", error: "Error", fatal: "Fatal", info: "Info", debug: "Debug", trace: "Trace" })[
    level ?? ""
  ] ?? "Info";

export const levelTone = (level?: string) =>
  level === "error" || level === "fatal" ? "down" : level === "warn" ? "degraded" : "unknown";

export const serviceRow = (entry: ApiLogEntry): LogRow => {
  const attributes = attributeSummary(entry.attributes);
  const component = (entry.attributes as { component?: string } | undefined)?.component;
  return {
    id: entry.id ?? "",
    at: entry.timestamp,
    level: entry.severity ?? "info",
    label: component ?? entry.app_name ?? (entry.source === "node" ? "Node" : undefined),
    title: entry.event ?? entry.body ?? "",
    detail: entry.event ? [entry.body, attributes].filter(Boolean).join(" ") : attributes,
    inferred: entry.severity_inferred,
    plain: !entry.event && !attributes,
  };
};

export const statusRow = (entry: ApiStatusLogEntry): LogRow => ({
  id: entry.id ?? "",
  at: entry.at,
  level: entry.level ?? "info",
  label: entry.server_name,
  monitor: entry.monitor_name,
  title: entry.title ?? "",
  detail: entry.detail,
});

/** Stable colour per app so the source dot reads the same everywhere. */
const PALETTE = ["#2a9f69", "#6366f1", "#f59e0b", "#0ea5e9", "#e5484d", "#a855f7", "#18181b"];
export const sourceColor = (label?: string) => {
  let hash = 0;
  for (const char of label ?? "") hash = (hash * 31 + char.charCodeAt(0)) >>> 0;
  return PALETTE[hash % PALETTE.length];
};
