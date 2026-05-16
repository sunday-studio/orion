import { type GetAgentsParams, useGetAgents } from "@/orion-sdk";
import { AgentRow } from "./agent-row";
import { ListPagination } from "@/components/list-pagination";
import { Separator } from "@/components/ui/separator";
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
  const agents = agentsResponse.data?.agents ?? [];
  const count = agentsResponse.data?.count ?? agents.length;

  const setOffset = (nextOffset: number) => {
    void setServerQuery({ page: Math.floor(nextOffset / AGENT_LIMIT) + 1 });
  };

  return (
    <div className="space-y-3">
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
      {agentsResponse.isLoading && (
        <div className="py-3 text-sm text-neutral-600">Loading agents...</div>
      )}
      {agentsResponse.error && <div className="py-3 text-sm">Unable to load agents.</div>}
      {!agentsResponse.isLoading && !agentsResponse.error && agents.length === 0 && (
        <div className="py-3 text-sm text-neutral-600">No agents match these filters.</div>
      )}
      <div className="space-y-1">
        {agents.map((agent, index) => (
          <Fragment key={agent.id}>
            <AgentRow agent={agent} />
            {index < agents.length - 1 && <Separator />}
          </Fragment>
        ))}
      </div>
      <ListPagination
        count={count}
        limit={AGENT_LIMIT}
        offset={offset}
        onOffsetChange={setOffset}
      />
    </div>
  );
};
