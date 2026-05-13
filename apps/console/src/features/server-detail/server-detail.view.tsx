import { PageBreadcrumbs } from "@/components/page-breadcrumbs";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  type ApiAgentReportResponse,
  useGetAgent,
  useGetAgentHealth,
  useGetAgentMonitors,
  useGetAgentReports,
  useGetIncidents,
} from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { useParams } from "react-router-dom";
import { useState } from "react";
import { AgentCpuTab } from "./components/agent-cpu-tab";
import { monitorPriority } from "./components/agent-detail-utils";
import { AgentLogsTab } from "./components/agent-logs-tab";
import { AgentMonitorsTab } from "./components/agent-monitors-tab";

const REPORT_LIMIT = 10;

const asLatestReport = (value: unknown): ApiAgentReportResponse => {
  if (!value || typeof value !== "object") return {};
  return value as ApiAgentReportResponse;
};

export const AgentDetailPage = () => {
  const { agentId = "", serverId = "" } = useParams();
  const currentAgentId = agentId || serverId;
  const [reportOffset, setReportOffset] = useState(0);
  const agentResponse = useGetAgent(currentAgentId);
  const healthResponse = useGetAgentHealth(currentAgentId);
  const monitorsResponse = useGetAgentMonitors(currentAgentId, { limit: 100 });
  const reportsResponse = useGetAgentReports(currentAgentId, {
    limit: REPORT_LIMIT,
    offset: reportOffset,
  });
  const incidentsResponse = useGetIncidents({ limit: 100 });

  const agent = agentResponse.data?.agent;
  const latestReport = asLatestReport(agentResponse.data?.latest_report);
  const reports = reportsResponse.data?.reports ?? [];
  const reportCount = reportsResponse.data?.count ?? reports.length;
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

      <Tabs defaultValue="logs" className="space-y-4">
        <TabsList variant="line">
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="monitors">Monitors</TabsTrigger>
          <TabsTrigger value="cpu">CPU</TabsTrigger>
        </TabsList>

        <TabsContent value="logs">
          <AgentLogsTab
            reports={reports}
            isLoading={reportsResponse.isLoading}
            hasError={Boolean(reportsResponse.error)}
            count={reportCount}
            limit={REPORT_LIMIT}
            offset={reportOffset}
            onOffsetChange={setReportOffset}
          />
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
          />
        </TabsContent>
      </Tabs>
    </div>
  );
};

export const ServerDetailPage = AgentDetailPage;
