import { Download, Plus, Search } from "lucide-react";
import { keepPreviousData } from "@tanstack/react-query";
import { useState } from "react";
import { Button, EmptyState, Input, PageHeader, Skeleton, usePageSize } from "@/design-system";
import { exportIncidentsCSV, useGetIncidents } from "@/orion-sdk";
import type { NumberedIncident } from "./incident-format";
import { DeclareIncidentDialog } from "./list/declare-incident-dialog";
import { IncidentBulkBar } from "./list/incident-bulk-bar";
import { IncidentFilterRow, useIncidentFilters } from "./list/incident-filters";
import { IncidentQuickViews } from "./list/incident-quick-views";
import { IncidentTable } from "./list/incident-table";

const PAGE_SIZE = 20;

const downloadBlob = (blob: Blob) => {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = "incidents.csv";
  link.click();
  URL.revokeObjectURL(url);
};

export const IncidentsPage = () => {
  const filters = useIncidentFilters();
  const [declaring, setDeclaring] = useState(false);
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [now] = useState(() => Date.now());

  const statuses = filters.statuses.filter((status) => status !== "none");
  const params = {
    status: statuses.length > 0 ? statuses.join(",") : "open,acknowledged,covered,resolved",
    severity: filters.severities.length > 0 ? filters.severities.join(",") : undefined,
    opened_since: filters.since || undefined,
    q: filters.q.trim() || undefined,
    notification_status: filters.notified.length > 0 ? filters.notified.join(",") : undefined,
    agent_id: filters.server || undefined,
    monitor_id: filters.monitor || undefined,
    needs_review: filters.review || undefined,
  };
  const { limit, setLimit } = usePageSize(PAGE_SIZE);
  const offset = (filters.page - 1) * limit;
  const query = useGetIncidents(
    { ...params, limit, offset },
    { query: { placeholderData: keepPreviousData } },
  );
  const incidents: NumberedIncident[] = query.data?.incidents ?? [];
  const total =
    (query.data as { pagination?: { total_items?: number } } | undefined)?.pagination
      ?.total_items ??
    query.data?.count ??
    incidents.length;
  const hiddenResolved =
    (query.data as { hidden_resolved_count?: number } | undefined)?.hidden_resolved_count ?? 0;
  const [exporting, setExporting] = useState(false);
  const exportCsv = async () => {
    setExporting(true);
    try {
      downloadBlob(await exportIncidentsCSV(params));
    } finally {
      setExporting(false);
    }
  };

  const view =
    filters.review || false
      ? "review"
      : statuses.length === 1 && statuses[0] === "resolved"
        ? "resolved"
        : "active";
  const isEmptyHistory = query.isSuccess && incidents.length === 0 && filters.isDefault;

  const toggle = (id: string) =>
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  return (
    <div>
      <PageHeader
        title="Incidents"
        description={isEmptyHistory ? undefined : "Everything Orion has opened, newest first."}
        actions={
          <>
            <Button
              size="sm"
              variant="secondary"
              disabled={exporting}
              onClick={() => void exportCsv()}
            >
              <Download /> Export CSV
            </Button>
            <Button size="sm" onClick={() => setDeclaring(true)}>
              <Plus /> Declare incident
            </Button>
          </>
        }
      />
      <div className="mt-3 flex flex-col gap-3">
        {!isEmptyHistory && <IncidentFilterRow filters={filters} />}
        {!isEmptyHistory && (
          <div className="flex items-center gap-3">
            <div className="relative flex-1">
              <Search
                aria-hidden
                className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-faint"
              />
              <Input
                aria-label="Search incidents"
                placeholder="Search incidents"
                className="pl-9"
                value={filters.q}
                onChange={(event) => filters.setQuery(event.target.value)}
              />
            </div>
            <IncidentQuickViews filters={filters} current={view} />
          </div>
        )}
        {query.isLoading ? (
          <Skeleton className="h-40 w-full rounded-xl" />
        ) : query.isError ? (
          <EmptyState
            tone="error"
            title="Unable to load incidents"
            action={
              <Button size="sm" variant="secondary" onClick={() => void query.refetch()}>
                Retry
              </Button>
            }
          />
        ) : isEmptyHistory ? (
          <div className="flex min-h-[207px] flex-col items-center justify-center gap-1 rounded-xl border border-hairline bg-card px-4 text-center shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
            <b className="text-[15px] font-medium">No open incidents</b>
            <p className="text-[13px] text-mid">
              Everything Orion watches is healthy. Resolved incidents are in the history.
            </p>
            <Button
              size="sm"
              variant="secondary"
              className="mt-3"
              onClick={() => filters.setStatuses(["open", "acknowledged", "covered", "resolved"])}
            >
              Show resolved incidents
            </Button>
          </div>
        ) : incidents.length === 0 ? (
          <EmptyState
            title="No incidents match these filters"
            description="Clear the filters to see everything Orion has opened."
            action={
              <Button size="sm" variant="secondary" onClick={filters.clearAll}>
                Clear all
              </Button>
            }
          />
        ) : (
          <IncidentTable
            incidents={incidents}
            selected={selected}
            onToggle={toggle}
            onToggleAll={(checked) =>
              setSelected(checked ? new Set(incidents.map((i) => i.id ?? "")) : new Set())
            }
            now={now}
            fetching={query.isPlaceholderData}
            hiddenResolved={hiddenResolved}
            onShowResolved={() => filters.setStatuses([...statuses, "resolved"])}
            pagination={{
              count: total,
              limit,
              offset,
              onOffsetChange: (next) => filters.setPage(Math.floor(next / limit) + 1),
              onLimitChange: setLimit,
            }}
          />
        )}
      </div>
      {selected.size > 0 && (
        <IncidentBulkBar ids={Array.from(selected)} onClear={() => setSelected(new Set())} />
      )}
      <DeclareIncidentDialog open={declaring} onOpenChange={setDeclaring} />
    </div>
  );
};
