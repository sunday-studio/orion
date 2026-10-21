import type { ApiMonitorResponse } from "@/orion-sdk";

export const monitorHealth = (monitor: ApiMonitorResponse) =>
  monitor.health ?? monitor.computed_health ?? "unknown";

export const monitorPriority = (monitor: ApiMonitorResponse) => {
  const health = monitorHealth(monitor);
  if (health === "down" || health === "degraded") return 0;
  if (health === "unknown" || health === "stale") return 1;
  return 2;
};
