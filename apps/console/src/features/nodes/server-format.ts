import type { ApiNodeResponse } from "@/orion-sdk";
import { formatSpan } from "@/features/overview/overview-time";

export type ServerRow = ApiNodeResponse;

export type ServerState = "up" | "degraded" | "down" | "stale" | "maintenance" | "unknown";

export const serverState = (node: ServerRow): ServerState => {
  if (node.maintenance_mode) return "maintenance";
  const status = (node.status ?? "").toLowerCase();
  if (status === "stale" || /stale/.test(node.status_reason ?? "")) return "stale";
  if (status === "up" || status === "down" || status === "degraded") return status;
  return "unknown";
};

const RANK: Record<ServerState, number> = {
  down: 0,
  stale: 1,
  degraded: 2,
  unknown: 3,
  maintenance: 4,
  up: 5,
};

export const byAttention = (a: ServerRow, b: ServerRow) =>
  RANK[serverState(a)] - RANK[serverState(b)] || (a.name ?? "").localeCompare(b.name ?? "");

export const needsAttention = (node: ServerRow) => {
  const state = serverState(node);
  return state === "down" || state === "stale" || state === "degraded";
};

export const stateDot: Record<ServerState, string> = {
  up: "#2a9f69",
  degraded: "#f59e0b",
  down: "#e5484d",
  stale: "#a1a1aa",
  maintenance: "#a1a1aa",
  unknown: "#a1a1aa",
};

export const platformLabel = (node: ServerRow) =>
  node.os && node.arch ? `${node.os}/${node.arch}` : (node.platform ?? node.os ?? "-");

/** "Up 41d", "No report 12m", "Disk 91%", "Down". */
export const statusText = (node: ServerRow, now: number) => {
  if (node.status_text) return node.status_text;
  const state = serverState(node);
  const seen = node.last_seen ? new Date(node.last_seen).getTime() : undefined;
  if (state === "up") {
    return node.uptime_seconds
      ? `Up ${formatSpan(0, node.uptime_seconds * 1000, false, true).split(" ")[0]}`
      : "Up";
  }
  if (state === "stale")
    return seen ? `No report ${formatSpan(seen, now, false, true)}` : "No report";
  if (state === "degraded") return node.status_detail ?? node.status_reason ?? "Degraded";
  if (state === "down") return node.status_detail ?? "Down";
  if (state === "maintenance") return "Maintenance";
  return "Unknown";
};

export const percent = (value?: number | null) =>
  typeof value === "number" ? `${Math.round(value)}%` : "-";
