import { useQueries } from "@tanstack/react-query";
import { EmptyState, GridRow, GridTable } from "@/design-system";
import { formatBytes } from "../timeline-model";
import {
  type ServiceServiceLinkedResource,
  getGetNodeAppResourcesQueryOptions,
  useGetNodes,
} from "@/orion-sdk";

const COLUMNS = "minmax(0,1.2fr) 100px 90px 110px 130px";

/** Latest CPU and memory for each linked resource, read from the host's last report. */
export const ServiceResourcesTab = ({
  resources,
}: {
  resources: readonly ServiceServiceLinkedResource[];
}) => {
  const nodeIds = [...new Set(resources.map((r) => r.node_id ?? ""))].filter(Boolean);
  const nodes = useGetNodes({ limit: 200 });
  const reports = useQueries({
    queries: nodeIds.map((id) => getGetNodeAppResourcesQueryOptions(id)),
  });
  if (resources.length === 0) {
    return (
      <EmptyState
        title="No resources linked"
        description="Link a process, unit or container from a server to see its CPU and memory here."
      />
    );
  }
  const latest = new Map(
    reports.flatMap((report, index) =>
      (report.data?.app_resources?.series ?? []).map(
        (series) => [`${nodeIds[index]}:${series.key}`, series] as const,
      ),
    ),
  );
  const nodeName = (id?: string) =>
    nodes.data?.nodes?.find((node) => node.id === id)?.name ?? id ?? "-";

  return (
    <GridTable className="overflow-hidden rounded-xl border border-hairline">
      <GridRow columns={COLUMNS} header>
        <span>Resource</span>
        <span>Kind</span>
        <span>CPU</span>
        <span>Memory</span>
        <span>Server</span>
      </GridRow>
      {resources.map((resource) => {
        const series = latest.get(`${resource.node_id}:${resource.key}`);
        return (
          <GridRow key={`${resource.node_id}:${resource.key}`} columns={COLUMNS}>
            <b className="truncate font-medium">{resource.name ?? resource.key}</b>
            <span className="font-mono text-xs text-faint">{resource.kind}</span>
            <span className="tabular-nums">
              {typeof series?.cpu_percent === "number" ? `${+series.cpu_percent.toFixed(1)}%` : "-"}
            </span>
            <span className="tabular-nums">
              {typeof series?.memory_bytes === "number" ? formatBytes(series.memory_bytes) : "-"}
            </span>
            <span className="text-faint">{nodeName(resource.node_id)}</span>
          </GridRow>
        );
      })}
    </GridTable>
  );
};
