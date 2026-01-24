import type { components } from "@orion/sdk";

type ApiEnvelope = components["schemas"]["ApiEnvelope"];

const base = (import.meta as unknown as { env: { VITE_API_BASE_URL?: string } }).env
  .VITE_API_BASE_URL ?? "/v1";

async function request<T>(path: string, params?: Record<string, string | number | undefined>): Promise<T> {
  const q = new URLSearchParams();
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== "") q.set(k, String(v));
    }
  }
  const url = `${base.replace(/\/$/, "")}${path}${q.toString() ? `?${q}` : ""}`;
  const res = await fetch(url);
  const body = (await res.json()) as ApiEnvelope & { data?: T };
  if (!res.ok) {
    throw new Error(body.error || body.message || `HTTP ${res.status}`);
  }
  if (!body.success || body.data === undefined) {
    throw new Error(body.message || "Request failed");
  }
  return body.data as T;
}

export type Agent = components["schemas"]["Agent"];
export type AgentReport = components["schemas"]["AgentReport"];
export type Monitor = components["schemas"]["Monitor"];
export type MonitorReport = components["schemas"]["MonitorReport"];
export type UptimeDayBucket = components["schemas"]["UptimeDayBucket"];

export type ListAgentsData = { agents?: Agent[]; count?: number; limit?: number; offset?: number };
export type GetAgentDetailData = { agent?: Agent; latest_report?: AgentReport | null };
export type GetAgentReportsData = { reports?: AgentReport[]; count?: number; limit?: number; offset?: number };
export type ListMonitorsData = { monitors?: Monitor[]; count?: number; limit?: number; offset?: number };
export type GetMonitorDetailData = { monitor?: Monitor; recent_reports?: MonitorReport[]; computed_health?: string };
export type GetMonitorHistoryData = { reports?: MonitorReport[]; count?: number; limit?: number; offset?: number };
export type GetUptimeData = { daily_buckets?: UptimeDayBucket[]; uptime_percent?: number };

export const api = {
  listAgents(p?: { search?: string; status?: string; last_seen?: string; uptime?: string; sort?: string; order?: "asc" | "desc"; limit?: number; offset?: number }) {
    return request<ListAgentsData>("/agents", p as Record<string, string | number | undefined>);
  },
  getAgentDetail(id: string) {
    return request<GetAgentDetailData>(`/agents/${id}`);
  },
  getAgentReports(id: string, p?: { limit?: number; offset?: number }) {
    return request<GetAgentReportsData>(`/agents/${id}/reports`, p as Record<string, number | undefined>);
  },
  getAgentUptime(id: string, p?: { period?: string }) {
    return request<GetUptimeData>(`/agents/${id}/uptime`, p as Record<string, string | undefined>);
  },
  listMonitors(agentId: string, p?: { health?: string; lifecycle?: string; limit?: number; offset?: number }) {
    return request<ListMonitorsData>(`/agents/${agentId}/monitors`, p as Record<string, string | number | undefined>);
  },
  getMonitorDetail(id: string) {
    return request<GetMonitorDetailData>(`/monitors/${id}`);
  },
  getMonitorHistory(id: string, p?: { limit?: number; offset?: number }) {
    return request<GetMonitorHistoryData>(`/monitors/${id}/history`, p as Record<string, number | undefined>);
  },
  getMonitorUptime(id: string, p?: { period?: string }) {
    return request<GetUptimeData>(`/monitors/${id}/uptime`, p as Record<string, string | undefined>);
  },
};
