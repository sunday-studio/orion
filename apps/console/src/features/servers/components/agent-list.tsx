import { useGetAgents, type GetAgentsParams } from "@/orion-sdk";
import { AgentFilters, type AttentionFilterValue } from "./agent-filters";
import { AgentRow } from "./agent-row";
import { Separator } from "@/components/ui/separator";
import { Fragment } from "react/jsx-runtime";
import { ListPagination } from "@/components/list-pagination";
import { parseAsBoolean, parseAsInteger, parseAsString, useQueryStates } from "nuqs";

const SERVER_LIMIT = 20;

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
  const offset = (currentPage - 1) * SERVER_LIMIT;

  const params: GetAgentsParams = {
    limit: SERVER_LIMIT,
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
  const hasFilters = search.trim() !== "" || status !== "all" || maintenance || stale || incidents;
  const selectedAttentionFilters: AttentionFilterValue[] = [
    maintenance ? "maintenance" : undefined,
    stale ? "stale" : undefined,
    incidents ? "incidents" : undefined,
  ].filter((value): value is AttentionFilterValue => value !== undefined);

  const setSearch = (value: string) => {
    void setServerQuery({ search: value, page: 1 });
  };

  const setStatus = (value: string) => {
    void setServerQuery({ status: value, page: 1 });
  };

  const setAttentionFilters = (values: AttentionFilterValue[]) => {
    const selected = new Set(values);
    void setServerQuery({
      maintenance: selected.has("maintenance"),
      stale: selected.has("stale"),
      incidents: selected.has("incidents"),
      page: 1,
    });
  };

  const clearFilters = () => {
    void setServerQuery({
      search: "",
      status: "all",
      maintenance: false,
      stale: false,
      incidents: false,
      page: 1,
    });
  };

  const setOffset = (nextOffset: number) => {
    void setServerQuery({ page: Math.floor(nextOffset / SERVER_LIMIT) + 1 });
  };

  return (
    <div className="space-y-3">
      <AgentFilters
        search={search}
        status={status}
        selectedAttentionFilters={selectedAttentionFilters}
        hasFilters={hasFilters}
        onSearchChange={setSearch}
        onStatusChange={setStatus}
        onAttentionFiltersChange={setAttentionFilters}
        onClear={clearFilters}
      />
      {agentsResponse.isLoading && (
        <div className="py-3 text-sm text-neutral-600">Loading servers...</div>
      )}
      {agentsResponse.error && <div className="py-3 text-sm">Unable to load servers.</div>}
      {!agentsResponse.isLoading && !agentsResponse.error && agents.length === 0 && (
        <div className="py-3 text-sm text-neutral-600">No servers match these filters.</div>
      )}
      {agents.map((agent, index) => (
        <Fragment key={agent.id}>
          <AgentRow agent={agent} />
          {index < agents.length - 1 && <Separator />}
        </Fragment>
      ))}
      <ListPagination
        count={count}
        limit={SERVER_LIMIT}
        offset={offset}
        onOffsetChange={setOffset}
      />
    </div>
  );
};
