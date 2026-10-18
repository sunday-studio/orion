import type { ApiAgentResponse } from "@/orion-sdk";
import { formatSpan } from "@/features/overview/overview-time";

/** Fields Core is adding to the agents list for F3; all optional until they ship. */
export type ServerRow = ApiAgentResponse & {
  cpu_percent?: number | null;
  memory_percent?: number | null;
  cpu_series?: number[] | null;
  status_detail?: string | null;
};

export type ServerState = "up" | "degraded" | "down" | "stale" | "maintenance" | "unknown";

export const serverState = (agent: ServerRow): ServerState => {
  if (agent.maintenance_mode) return "maintenance";
  const status = (agent.status ?? "").toLowerCase();
  if (status === "stale" || /stale/.test(agent.status_reason ?? "")) return "stale";
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

export const needsAttention = (agent: ServerRow) => {
  const state = serverState(agent);
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

export const platformLabel = (agent: ServerRow) =>
  agent.os && agent.arch ? `${agent.os}/${agent.arch}` : (agent.platform ?? agent.os ?? "-");

/** "Up 41d", "No report 12m", "Disk 91%", "Down". */
export const statusText = (agent: ServerRow, now: number) => {
  const state = serverState(agent);
  const seen = agent.last_seen ? new Date(agent.last_seen).getTime() : undefined;
  if (state === "up") {
    return agent.uptime_seconds
      ? `Up ${formatSpan(0, agent.uptime_seconds * 1000, false, true).split(" ")[0]}`
      : "Up";
  }
  if (state === "stale")
    return seen ? `No report ${formatSpan(seen, now, false, true)}` : "No report";
  if (state === "degraded") return agent.status_detail ?? agent.status_reason ?? "Degraded";
  if (state === "down") return agent.status_detail ?? "Down";
  if (state === "maintenance") return "Maintenance";
  return "Unknown";
};

export const percent = (value?: number | null) =>
  typeof value === "number" ? `${Math.round(value)}%` : "-";
