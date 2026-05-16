import { PageBreadcrumbs } from "@/components/page-breadcrumbs";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  type ApiAgentReportResponse,
  useGetAgent,
  useGetAgentHealth,
  useGetAgentUptime,
  useGetIncident,
  useGetIncidents,
} from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { useCallback, useEffect } from "react";
import { Link, useLocation, useParams, useSearchParams } from "react-router-dom";
import { AgentCpuTab } from "./components/agent-cpu-tab";
import { AgentHealthSummary } from "./components/agent-health-summary";
import { AgentLogsTab } from "./components/agent-logs-tab";
import { AgentMonitorsTab } from "./components/agent-monitors-tab";
import { PageHeader } from "@/components/page-header";
import { StatusBadge, toStatus } from "@/components/status-badges";

const AGENT_DETAIL_TABS = ["logs", "monitors", "cpu"] as const;
type AgentDetailTab = (typeof AGENT_DETAIL_TABS)[number];

const isAgentDetailTab = (value: string | null): value is AgentDetailTab =>
  AGENT_DETAIL_TABS.includes(value as AgentDetailTab);

const asLatestReport = (value: unknown): ApiAgentReportResponse => {
  if (!value || typeof value !== "object") return {};
  return value as ApiAgentReportResponse;
};

export const AgentDetailPage = () => {
  const { agentId = "" } = useParams();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const currentAgentId = agentId;
  const selectedTab = searchParams.get("tab");
  const highlightedIncidentId = searchParams.get("incident") ?? "";
  const activeTab = isAgentDetailTab(selectedTab) ? selectedTab : "logs";
  const agentResponse = useGetAgent(currentAgentId);
  const healthResponse = useGetAgentHealth(currentAgentId);
  const uptimeResponse = useGetAgentUptime(currentAgentId, { period: "90d" });
  const incidentsResponse = useGetIncidents({ limit: 100 });
  const highlightedIncidentResponse = useGetIncident(highlightedIncidentId);

  const agent = agentResponse.data?.agent;
  const latestReport = asLatestReport(agentResponse.data?.latest_report);
  const activeIncidents = (incidentsResponse.data?.incidents ?? []).filter(
    (incident) => incident.agent_id === currentAgentId,
  );
  const highlightedIncidentFromList = activeIncidents.find(
    (incident) => incident.id === highlightedIncidentId,
  );
  const highlightedIncident =
    highlightedIncidentResponse.data?.incident?.agent_id === currentAgentId
      ? highlightedIncidentResponse.data.incident
      : highlightedIncidentFromList;
  const primaryIncident = highlightedIncident ?? activeIncidents[0];
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
      <div className="space-y-2">
        <PageBreadcrumbs
          items={[{ label: "Agents", to: "/agents" }, { label: agent.name ?? "Agent" }]}
        />
        <div className="flex flex-wrap items-start justify-between gap-1 flex-col ">
          <PageHeader
            title={agent.name ?? agent.id ?? "Unknown agent"}
            description={
              <p className="text-sm text-neutral-600">
                <StatusBadge className="text-sm px-1.5 py-1 capitalize" value={toStatus(status)} />{" "}
                · last update {formatDate(agent.last_seen, DATE_TIME_FORMAT)}
              </p>
            }
          />
        </div>
      </div>

      {highlightedIncident && (
        <section className="space-y-1 bg-amber-50 px-3 py-2 text-sm">
          <div className="font-medium">
            Highlighted incident: {highlightedIncident.title ?? highlightedIncident.id}
          </div>
          <div className="text-neutral-700">
            {highlightedIncident.latest_event ?? "No latest event recorded."}
          </div>
        </section>
      )}

      {/* {!highlightedIncident && primaryIncident && (
        <section className="flex flex-wrap items-center justify-between gap-3 bg-rose-100 px-3 py-4 text-sm">
          <div>
            <div className="font-medium text-rose-900">
              Active incident: {primaryIncident.title ?? primaryIncident.id}
            </div>
            <div className="text-neutral-600">
              {primaryIncident.latest_event ?? "No latest event recorded."}
            </div>
          </div>
          <Link
            className="font-medium hover:text-rose-900 px-2 py-1.5 hover:bg-rose-200"
            to={`/incidents/${primaryIncident.id}`}
          >
            View incident
          </Link>
        </section>
      )} */}
      {/*
      <AgentHealthSummary
        activeIncidentCount={activeIncidents.length}
        degradedCount={healthResponse.data?.degraded_count ?? 0}
        downCount={healthResponse.data?.down_count ?? 0}
        status={status}
        upCount={healthResponse.data?.up_count ?? 0}
        uptimePercent={uptimeResponse.data?.uptime_percent}
        uptimeBuckets={uptimeResponse.data?.daily_buckets ?? []}
      />

      <Tabs value={activeTab} onValueChange={handleTabChange} className="space-y-4">
        <TabsList variant="line">
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="monitors">Monitors</TabsTrigger>
          <TabsTrigger value="cpu">System</TabsTrigger>
        </TabsList>

        <TabsContent value="logs">
          <AgentLogsTab agentId={currentAgentId} />
        </TabsContent>

        <TabsContent value="monitors">
          <AgentMonitorsTab agentId={currentAgentId} highlightedIncident={highlightedIncident} />
        </TabsContent>

        <TabsContent value="cpu">
          <AgentCpuTab agent={agent} latestReport={latestReport} configSummary={configSummary} />
        </TabsContent>
      </Tabs> */}
    </div>
  );
};

export const ServerDetailPage = AgentDetailPage;
