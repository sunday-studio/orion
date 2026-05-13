import { PageBreadcrumbs } from "@/components/page-breadcrumbs";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  type ApiAgentReportResponse,
  useGetAgent,
  useGetAgentHealth,
  useGetAgentMonitors,
  useGetAgentUptime,
  useGetIncidents,
} from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { useCallback, useEffect } from "react";
import { useLocation, useParams, useSearchParams } from "react-router-dom";
import { AgentCpuTab } from "./components/agent-cpu-tab";
import { monitorPriority } from "./components/agent-detail-utils";
import { AgentLogsTab } from "./components/agent-logs-tab";
import { AgentMonitorsTab } from "./components/agent-monitors-tab";

const AGENT_DETAIL_TABS = ["logs", "monitors", "cpu"] as const;
type AgentDetailTab = (typeof AGENT_DETAIL_TABS)[number];

const isAgentDetailTab = (value: string | null): value is AgentDetailTab =>
  AGENT_DETAIL_TABS.includes(value as AgentDetailTab);

const asLatestReport = (value: unknown): ApiAgentReportResponse => {
  if (!value || typeof value !== "object") return {};
  return value as ApiAgentReportResponse;
};

export const AgentDetailPage = () => {
  const { agentId = "", serverId = "" } = useParams();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const currentAgentId = agentId || serverId;
  const selectedTab = searchParams.get("tab");
  const activeTab = isAgentDetailTab(selectedTab) ? selectedTab : "logs";
  const agentResponse = useGetAgent(currentAgentId);
  const healthResponse = useGetAgentHealth(currentAgentId);
  const monitorsResponse = useGetAgentMonitors(currentAgentId, { limit: 100 });
  const uptimeResponse = useGetAgentUptime(currentAgentId, { period: "90d" });
  const incidentsResponse = useGetIncidents({ limit: 100 });

  const agent = agentResponse.data?.agent;
  const latestReport = asLatestReport(agentResponse.data?.latest_report);
  const monitors = [...(monitorsResponse.data?.monitors ?? [])].sort(
    (left, right) => monitorPriority(left) - monitorPriority(right),
  );
  const activeIncidents = (incidentsResponse.data?.incidents ?? []).filter(
    (incident) => incident.agent_id === currentAgentId,
  );
  const status =
    healthResponse.data?.overall_health ??
    agent?.status ??
    (agent?.maintenance_mode ? "maintenance" : "unknown");
  const configSummary =
    typeof latestReport.config_summary === "string"
      ? latestReport.config_summary
      : JSON.stringify(latestReport.config_summary ?? {}, null, 2);
  const scrollKey = useCallback(
    (tab: AgentDetailTab) => `orion:scroll:${location.pathname}:${tab}`,
    [location.pathname],
  );
  const saveScroll = useCallback(
    (tab: AgentDetailTab) => {
      window.sessionStorage.setItem(scrollKey(tab), String(window.scrollY));
    },
    [scrollKey],
  );
  const handleTabChange = useCallback(
    (value: string) => {
      if (!isAgentDetailTab(value)) return;
      saveScroll(activeTab);
      setSearchParams((params) => {
        const nextParams = new URLSearchParams(params);
        if (value === "logs") {
          nextParams.delete("tab");
        } else {
          nextParams.set("tab", value);
        }
        return nextParams;
      });
    },
    [activeTab, saveScroll, setSearchParams],
  );

  useEffect(() => {
    const savedScroll = Number(window.sessionStorage.getItem(scrollKey(activeTab)));
    if (!Number.isFinite(savedScroll) || savedScroll <= 0) return;

    const frame = window.requestAnimationFrame(() => {
      window.scrollTo(0, savedScroll);
    });

    return () => window.cancelAnimationFrame(frame);
  }, [activeTab, scrollKey]);

  useEffect(() => {
    return () => saveScroll(activeTab);
  }, [activeTab, saveScroll]);

  if (agentResponse.isLoading) {
    return <div className="py-3 text-sm text-neutral-600">Loading agent...</div>;
  }

  if (agentResponse.error || !agent) {
    return <div className="py-3 text-sm">Unable to load agent.</div>;
  }

  return (
    <div className="space-y-7">
      <div className="space-y-1">
        <PageBreadcrumbs
          items={[{ label: "Agents", to: "/agents" }, { label: agent.name ?? "Agent" }]}
        />
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="text-base font-medium">{agent.name ?? agent.id ?? "Unknown agent"}</h1>
            <p className="text-sm text-neutral-600">
              {status} · last seen {formatDate(agent.last_seen, DATE_TIME_FORMAT)}
            </p>
          </div>
          {agent.maintenance_mode && <div className="text-sm font-medium">maintenance</div>}
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={handleTabChange} className="space-y-4">
        <TabsList variant="line">
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="monitors">Monitors</TabsTrigger>
          <TabsTrigger value="cpu">CPU</TabsTrigger>
        </TabsList>

        <TabsContent value="logs">
          <AgentLogsTab agentId={currentAgentId} />
        </TabsContent>

        <TabsContent value="monitors">
          <AgentMonitorsTab
            monitors={monitors}
            isLoading={monitorsResponse.isLoading}
            hasError={Boolean(monitorsResponse.error)}
          />
        </TabsContent>

        <TabsContent value="cpu">
          <AgentCpuTab
            agent={agent}
            latestReport={latestReport}
            status={status}
            upCount={healthResponse.data?.up_count ?? 0}
            downCount={healthResponse.data?.down_count ?? 0}
            degradedCount={healthResponse.data?.degraded_count ?? 0}
            activeIncidentCount={activeIncidents.length}
            configSummary={configSummary}
            uptimePercent={uptimeResponse.data?.uptime_percent}
            uptimeBuckets={uptimeResponse.data?.daily_buckets ?? []}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
};

export const ServerDetailPage = AgentDetailPage;
