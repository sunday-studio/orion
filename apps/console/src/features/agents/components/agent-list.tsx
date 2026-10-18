import { type GetAgentsParams, useGetAgentSummary, useGetAgents } from "@/orion-sdk";
import { AgentRow } from "./agent-row";
import { AgentSummary, type AgentSummaryFilter } from "./agent-summary";
import { Button, EmptyState, ListPagination, Skeleton, usePageSize } from "@/design-system";
import { parseAsBoolean, parseAsInteger, parseAsString, useQueryStates } from "nuqs";
import { Fragment } from "react/jsx-runtime";

export const AgentList = () => {
  const [{ search, status, maintenance, stale, incidents, page }, setServerQuery] = useQueryStates({
    search: parseAsString.withDefault(""),
    status: parseAsString.withDefault("all"),
    maintenance: parseAsBoolean.withDefault(false),
    stale: parseAsBoolean.withDefault(false),
    incidents: parseAsBoolean.withDefault(false),
    page: parseAsInteger.withDefault(1),
  });
  const currentPage = Math.max(page, 1);
  const { limit: pageSize, setLimit: setPageSize } = usePageSize(20, {
    key: "pageSize",
    pageKey: "page",
  });
  const offset = (currentPage - 1) * pageSize;

  const params: GetAgentsParams = {
    limit: pageSize,
    offset,
    search: search.trim() || undefined,
    status: status === "all" ? undefined : status,
    maintenance: maintenance ? "true" : undefined,
    stale_only: stale || undefined,
    has_incidents: incidents || undefined,
  };

  const agentsResponse = useGetAgents(params);
  const summaryResponse = useGetAgentSummary();
  const agents = agentsResponse.data?.agents ?? [];
  const count = agentsResponse.data?.count ?? agents.length;
  const selectedSummaryFilter: AgentSummaryFilter = incidents
    ? "incidents"
    : stale
      ? "stale"
      : maintenance
        ? "maintenance"
        : status === "up" || status === "down" || status === "degraded" || status === "unknown"
          ? status
          : "all";

  const setOffset = (nextOffset: number) => {
    void setServerQuery({ page: Math.floor(nextOffset / pageSize) + 1 });
  };

  const setSummaryFilter = (filter: AgentSummaryFilter) => {
    void setServerQuery({
      status: ["up", "down", "degraded", "unknown"].includes(filter) ? filter : "all",
      maintenance: filter === "maintenance",
      stale: filter === "stale",
      incidents: filter === "incidents",
      page: 1,
    });
  };

  return (
    <div>
      <AgentSummary
        summary={summaryResponse.data}
        selectedFilter={selectedSummaryFilter}
        onFilterChange={setSummaryFilter}
      />
      {summaryResponse.error && (
        <EmptyState
          className="min-h-32"
          title="Unable to load server summary"
          description="The server list can still load, but summary filters may be stale."
          tone="error"
          action={
            <Button size="sm" variant="secondary" onClick={() => void summaryResponse.refetch()}>
              Retry
            </Button>
          }
        />
      )}
      {agentsResponse.isLoading && (
        <div aria-busy className="space-y-2 py-3">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      )}
      {agentsResponse.error && (
        <EmptyState
          title="Unable to load servers"
          description="Retry after Core is reachable."
          tone="error"
          action={
            <Button size="sm" variant="secondary" onClick={() => void agentsResponse.refetch()}>
              Retry
            </Button>
          }
        />
      )}
      {!agentsResponse.isLoading && !agentsResponse.error && agents.length === 0 && (
        <EmptyState
          title="No servers found"
          description="No installed servers match the current filters."
        />
      )}
      <div className=" my-6">
        {agents.map((agent, index) => (
          <Fragment key={agent.id}>
            <AgentRow agent={agent} index={index} />
          </Fragment>
        ))}
      </div>
      {count > 0 && (
        <ListPagination
          onLimitChange={setPageSize}
          count={count}
          limit={pageSize}
          offset={offset}
          onOffsetChange={setOffset}
        />
      )}
    </div>
  );
};
