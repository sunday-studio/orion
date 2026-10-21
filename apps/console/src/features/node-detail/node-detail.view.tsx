import { useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import {
  Callout,
  EmptyState,
  Skeleton,
  TabCount,
  Tabs,
  TabsList,
  TabsPanel,
  TabsTab,
} from "@/design-system";
import { type ServerRow, serverState } from "@/features/nodes/server-format";
import { useNow } from "@/features/overview/overview-time";
import { incidentLabel } from "@/features/incidents/incident-format";
import {
  type ApiNodeReportResponse,
  useGetNode,
  useGetNodeDiscovery,
  useGetIncidents,
} from "@/orion-sdk";
import { NodeLogsTab } from "./components/node-logs-tab";
import { AppsTab } from "./components/apps-tab";
import { DetectedTab } from "./components/detected-tab";
import { NodeMonitorsTab } from "./components/node-monitors-tab";
import { ServerEditDialog } from "./components/server-edit-dialog";
import { ServerHeader } from "./components/server-header";
import { ServerOverviewTab } from "./components/server-overview-tab";
import { ServerRemoveDialog } from "./components/server-remove-dialog";
import { ServerTokenTab } from "./components/server-token-tab";

const TABS = ["overview", "reports", "monitors", "apps", "detected", "token"] as const;
type Tab = (typeof TABS)[number];

/** Old links used "logs" and "cpu"; they land on the matching new tab. */
const resolveTab = (value: string | null): Tab => {
  if (value === "logs") return "reports";
  if (TABS.includes(value as Tab)) return value as Tab;
  return "overview";
};

export const NodeDetailPage = () => {
  const { nodeId = "" } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const tab = resolveTab(searchParams.get("tab"));
  const now = useNow(30_000);
  const query = useGetNode(nodeId);
  const discovery = useGetNodeDiscovery(nodeId).data?.discovery;
  const [editing, setEditing] = useState(false);
  const [removing, setRemoving] = useState(false);

  const incidents = useGetIncidents({
    node_id: nodeId,
    status: "open,acknowledged,covered",
    limit: 1,
  });
  const activeIncident = incidents.data?.incidents?.[0];
  const node = query.data?.node as ServerRow | undefined;
  const latest = (query.data?.latest_report ?? undefined) as ApiNodeReportResponse | undefined;

  const setTab = (next: string) =>
    setSearchParams(
      (params) => {
        const copy = new URLSearchParams(params);
        if (next === "overview") copy.delete("tab");
        else copy.set("tab", next);
        return copy;
      },
      { replace: true },
    );

  if (query.isLoading) return <Skeleton className="mt-10 h-40 w-full" />;
  if (query.isError || !node) {
    return (
      <EmptyState
        tone="error"
        title="Unable to load server"
        description="It may have been removed, or Core is unreachable."
      />
    );
  }

  return (
    <div className="w-full pb-8">
      <ServerHeader
        node={node}
        nodeVersion={node.node_version ?? latest?.node_version}
        now={now}
        onEdit={() => setEditing(true)}
        onToken={() => setTab("token")}
      />
      {activeIncident && (
        <Callout tone="down" className="mt-2 flex items-center justify-between gap-3">
          <span>
            <b className="mr-1.5 font-semibold">{incidentLabel(activeIncident)}</b>
            {activeIncident.title}
          </span>
          <Link className="shrink-0 underline" to={`/incidents/${activeIncident.id}`}>
            View incident
          </Link>
        </Callout>
      )}
      <Tabs value={tab} onValueChange={setTab} className="mt-3 gap-3">
        <TabsList>
          <TabsTab value="overview">Overview</TabsTab>
          <TabsTab value="reports">Reports</TabsTab>
          <TabsTab value="monitors">Monitors</TabsTab>
          <TabsTab value="apps">Apps</TabsTab>
          <TabsTab value="detected">
            Detected
            {(discovery?.suggested ?? 0) > 0 && <TabCount>{discovery?.suggested}</TabCount>}
          </TabsTab>
          <TabsTab value="token">Token</TabsTab>
        </TabsList>
        <TabsPanel value="overview">
          <ServerOverviewTab
            node={node}
            latest={latest}
            onOpenMonitors={() => setTab("monitors")}
          />
        </TabsPanel>
        <TabsPanel value="reports">
          <NodeLogsTab nodeId={nodeId} />
        </TabsPanel>
        <TabsPanel value="monitors">
          <NodeMonitorsTab nodeId={nodeId} />
        </TabsPanel>
        <TabsPanel value="apps">
          <AppsTab
            nodeId={nodeId}
            offline={["stale", "down"].includes(serverState(node))}
            onOpenDetected={() => setTab("detected")}
          />
        </TabsPanel>
        <TabsPanel value="detected">
          <DetectedTab nodeId={nodeId} onOpenApps={() => setTab("apps")} />
        </TabsPanel>
        <TabsPanel value="token">
          <ServerTokenTab nodeId={nodeId} name={node.name ?? nodeId} />
        </TabsPanel>
      </Tabs>
      <ServerEditDialog
        open={editing}
        node={node}
        onOpenChange={setEditing}
        onRemove={() => {
          setEditing(false);
          setRemoving(true);
        }}
      />
      <ServerRemoveDialog open={removing} node={node} onOpenChange={setRemoving} />
    </div>
  );
};
