import type { ApiMonitorResponse } from "@/orion-sdk";

export const formatPercent = (value?: number) =>
  typeof value === "number" ? `${value.toFixed(1)}%` : "—";

export const formatBytes = (value?: number) => {
  if (typeof value !== "number") return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size = size / 1024;
    unitIndex += 1;
  }
  return `${size.toFixed(size >= 10 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
};

export const formatDuration = (seconds?: number) => {
  if (typeof seconds !== "number") return "—";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  if (days > 0) return `${days}d ${hours}h`;
  return `${hours}h`;
};

export const monitorHealth = (monitor: ApiMonitorResponse) =>
  monitor.health ?? monitor.computed_health ?? "unknown";

export const monitorPriority = (monitor: ApiMonitorResponse) => {
  const health = monitorHealth(monitor);
  if (health === "down" || health === "degraded") return 0;
  if (health === "unknown" || health === "stale") return 1;
  return 2;
};
