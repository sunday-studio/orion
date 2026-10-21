import type { ApiMonitorResponse } from "@/orion-sdk";

export type MonitorRow = ApiMonitorResponse;

export const rowHealth = (monitor: MonitorRow) => {
  if (monitor.paused) return "paused";
  return monitor.health ?? monitor.computed_health ?? "unknown";
};

export const rowEvery = (monitor: MonitorRow) =>
  everyLabel(monitor.interval_seconds ?? monitor.reporting_interval_seconds);

export const everyLabel = (seconds?: number) => {
  if (!seconds) return "-";
  if (seconds % 86400 === 0) return `${seconds / 86400}d`;
  if (seconds % 3600 === 0) return `${seconds / 3600}h`;
  if (seconds % 60 === 0) return `${seconds / 60}m`;
  return `${seconds}s`;
};

const ACRONYMS = new Set([
  "http",
  "https",
  "tcp",
  "udp",
  "dns",
  "tls",
  "icmp",
  "ssl",
  "smtp",
  "imap",
  "pop",
]);

/** "http-healthcheck" -> "HTTP", "docker-container" -> "Docker". */
export const typeFamily = (type?: string) => {
  if (!type) return "-";
  const family = type.toLowerCase().split(/[-_ ]/)[0] ?? type;
  return ACRONYMS.has(family)
    ? family.toUpperCase()
    : family.charAt(0).toUpperCase() + family.slice(1);
};

export const statusLabel = (monitor: MonitorRow) => {
  if (monitor.status_text) return monitor.status_text;
  const health = rowHealth(monitor);
  return health.charAt(0).toUpperCase() + health.slice(1);
};

/** Target column: Core's text when present, otherwise the owner. */
export const targetLabel = (monitor: MonitorRow) =>
  monitor.target ??
  (monitor.owner_name ? `${typeFamily(monitor.type)}, ${monitor.owner_name}` : "-");
