import { Download, Plus, Search } from "lucide-react";
import { useMemo, useState } from "react";
import { Button, EmptyState, Input, PageHeader, Skeleton } from "@/design-system";
import { type ApiIncidentResponse, useGetIncidents } from "@/orion-sdk";
import { type NumberedIncident, sinceOptions } from "./incident-format";
import { DeclareIncidentDialog } from "./list/declare-incident-dialog";
import { IncidentBulkBar } from "./list/incident-bulk-bar";
import { IncidentFilterRow, useIncidentFilters } from "./list/incident-filters";
import { IncidentTable } from "./list/incident-table";

const PAGE_SIZE = 50;

const csvEscape = (value: string) => `"${value.replace(/"/g, '""')}"`;

const exportCsv = (rows: ApiIncidentResponse[]) => {
  const lines = [
    ["id", "title", "status", "severity", "monitor", "opened_at"].join(","),
    ...rows.map((incident) =>
      [
        incident.id,
        incident.title,
        incident.status,
        incident.severity,
        incident.monitor_name,
        incident.opened_at,
      ]
        .map((cell) => csvEscape(String(cell ?? "")))
        .join(","),
    ),
  ];
  const url = URL.createObjectURL(new Blob([lines.join("\n")], { type: "text/csv" }));
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
  const query = useGetIncidents({
    status: statuses.length > 0 ? statuses.join(",") : "open,acknowledged,covered,resolved",
    limit: PAGE_SIZE,
  });
  const resolvedHidden = useGetIncidents(
    { status: "resolved", limit: 1 },
    { query: { enabled: !statuses.includes("resolved") } },
  );

  const sinceMs = sinceOptions.find((option) => option.value === filters.since)?.ms;
  const text = filters.q.trim().toLowerCase();
  const incidents = useMemo<NumberedIncident[]>(
    () =>
      (query.data?.incidents ?? []).filter((incident) => {
        if (sinceMs && incident.opened_at && now - new Date(incident.opened_at).getTime() > sinceMs)
          return false;
        if (filters.severities.length > 0 && !filters.severities.includes(incident.severity ?? ""))
          return false;
        if (text && !(incident.title ?? "").toLowerCase().includes(text)) return false;
        return true;
      }),
    [query.data, sinceMs, filters.severities, text, now],
  );

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
            <Button size="sm" variant="secondary" onClick={() => exportCsv(incidents)}>
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
          <div className="relative">
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
            hiddenResolved={resolvedHidden.data?.count ?? 0}
            onShowResolved={() => filters.setStatuses([...statuses, "resolved"])}
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
