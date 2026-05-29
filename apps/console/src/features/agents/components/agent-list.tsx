import { type GetAgentsParams, useGetAgentSummary, useGetAgents } from "@/orion-sdk";
import { AgentRow } from "./agent-row";
import { AgentSummary, type AgentSummaryFilter } from "./agent-summary";
import { EmptyState } from "@/components/empty-state";
import { ListPagination } from "@/components/list-pagination";
import { parseAsBoolean, parseAsInteger, parseAsString, useQueryStates } from "nuqs";
import { Fragment } from "react/jsx-runtime";

const AGENT_LIMIT = 20;

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
  const offset = (currentPage - 1) * AGENT_LIMIT;

  const params: GetAgentsParams = {
    limit: AGENT_LIMIT,
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
    void setServerQuery({ page: Math.floor(nextOffset / AGENT_LIMIT) + 1 });
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
      {/* <AgentFilters
        search={search}
        status={status}
        selectedAttentionFilters={selectedAttentionFilters}
        hasFilters={hasFilters}
        onSearchChange={setSearch}
        onStatusChange={setStatus}
        onAttentionFiltersChange={setAttentionFilters}
        onClear={clearFilters}
      /> */}
      <AgentSummary
        summary={summaryResponse.data}
        selectedFilter={selectedSummaryFilter}
        onFilterChange={setSummaryFilter}
      />
      {summaryResponse.error && <div className="py-3 text-sm">Unable to load server summary.</div>}
      {agentsResponse.isLoading && (
        <div className="py-3 text-sm text-neutral-600">Loading servers...</div>
      )}
      {agentsResponse.error && <div className="py-3 text-sm">Unable to load servers.</div>}
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
          count={count}
          limit={AGENT_LIMIT}
          offset={offset}
          onOffsetChange={setOffset}
        />
      )}
    </div>
  );
};
