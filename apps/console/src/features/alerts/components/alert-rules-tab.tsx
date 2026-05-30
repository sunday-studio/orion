import { DataTable } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { SeverityBadge, toSeverity } from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger } from "@/components/ui/select";
import {
  type ApiAlertRouteResponse,
  type ApiAlertRuleResponse,
  useDryRunAlertRoutes,
  useGetAlertRoutes,
} from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import { useMemo, useState } from "react";
import { boolLabel, eventLabel } from "./alert-constants";

const ruleColumns: ColumnDef<ApiAlertRuleResponse>[] = [
  {
    accessorKey: "name",
    header: "Name",
    cell: ({ row }) => <span className="font-medium">{row.original.name ?? "unnamed"}</span>,
  },
  {
    accessorKey: "trigger_condition",
    header: "Trigger",
    cell: ({ row }) => (
      <div className="max-w-88 truncate text-neutral-600">
        {row.original.trigger_condition ?? "—"}
      </div>
    ),
  },
  {
    accessorKey: "severity",
    header: "Severity",
    cell: ({ row }) => <SeverityBadge value={toSeverity(row.original.severity)} />,
  },
  {
    accessorKey: "cooldown_seconds",
    header: "Cooldown",
    cell: ({ row }) => `${row.original.cooldown_seconds ?? 0}s`,
  },
  {
    accessorKey: "recovery_notification_enabled",
    header: "Recovery",
    cell: ({ row }) => boolLabel(row.original.recovery_notification_enabled),
  },
  {
    accessorKey: "target_channels",
    header: "Destinations",
    cell: ({ row }) => (row.original.target_channels ?? []).join(", ") || "none",
  },
];

const routeColumns: ColumnDef<ApiAlertRouteResponse>[] = [
  {
    accessorKey: "priority",
    header: "Priority",
    cell: ({ row }) => row.original.priority ?? 0,
  },
  {
    accessorKey: "name",
    header: "Route",
    cell: ({ row }) => <span className="font-medium">{row.original.name ?? "unnamed"}</span>,
  },
  {
    accessorKey: "enabled",
    header: "Enabled",
    cell: ({ row }) => boolLabel(row.original.enabled),
  },
  {
    accessorKey: "event_types",
    header: "Events",
    cell: ({ row }) => (
      <div className="max-w-72 truncate text-neutral-600">
        {(row.original.event_types ?? []).map(eventLabel).join(", ") || "all events"}
      </div>
    ),
  },
  {
    id: "filters",
    header: "Filters",
    cell: ({ row }) => {
      const route = row.original;
      const filters = [
        ...(route.severities?.length ? [`severity ${route.severities.join(",")}`] : []),
        ...(route.monitor_types?.length ? [`type ${route.monitor_types.join(",")}`] : []),
        ...(route.agent_ids?.length ? [`servers ${route.agent_ids.length}`] : []),
        ...(route.monitor_ids?.length ? [`monitors ${route.monitor_ids.length}`] : []),
      ];
      return (
        <div className="max-w-80 truncate text-neutral-600">{filters.join("; ") || "none"}</div>
      );
    },
  },
  {
    id: "destinations",
    header: "Destinations",
    cell: ({ row }) =>
      row.original.suppress ? "suppresses" : `${row.original.channel_ids?.length ?? 0} configured`,
  },
  {
    id: "grouping",
    header: "Grouping",
    cell: ({ row }) =>
      `${row.original.grouping_policy ?? "suppress"} / ${row.original.grouping_delay_seconds ?? 0}s`,
  },
];

type AlertRulesTabProps = {
  error: unknown;
  isLoading: boolean;
  rules: ApiAlertRuleResponse[];
};

export const AlertRulesTab = ({ error, isLoading, rules }: AlertRulesTabProps) => {
  const columns = useMemo(() => ruleColumns, []);
  const routeTableColumns = useMemo(() => routeColumns, []);
  const routesQuery = useGetAlertRoutes();
  const routes = routesQuery.data?.routes ?? [];
  const dryRunRoutes = useDryRunAlertRoutes();
  const [dryRunIncidentId, setDryRunIncidentId] = useState("");
  const [dryRunEventType, setDryRunEventType] = useState("incident_opened");
  const [dryRunSeverity, setDryRunSeverity] = useState("high");
  const [dryRunMonitorType, setDryRunMonitorType] = useState("http");
  const dryRun = dryRunRoutes.data?.dry_run;
  const runDryRun = () => {
    dryRunRoutes.mutate({
      data: {
        event_type: dryRunEventType,
        incident_id: dryRunIncidentId.trim() || undefined,
        monitor_type: dryRunMonitorType.trim() || undefined,
        severity: dryRunSeverity,
      },
    });
  };

  return (
    <section className="space-y-7">
      <div className="space-y-3">
        <div>
          <h2 className="text-sm font-medium">Rules</h2>
          <p className="text-sm text-neutral-600">
            These are the effective alert rules Core applies during incident reconciliation.
          </p>
        </div>
        {Boolean(error) && (
          <EmptyState
            className="min-h-32"
            title="Unable to load alert rules"
            description="Routes can still be inspected below if they loaded."
            tone="error"
          />
        )}
        {!error && (
          <DataTable
            columns={columns}
            data={rules}
            emptyMessage="No alert rules configured."
            getRowId={(rule, index) => rule.name ?? `rule-${index}`}
            isLoading={isLoading}
            loadingMessage="Loading alert rules..."
          />
        )}
      </div>

      <div className="space-y-3">
        <div>
          <h2 className="text-sm font-medium">Routes</h2>
          <p className="text-sm text-neutral-600">
            Ordered route matches, suppressions, grouping, and destination counts.
          </p>
        </div>
        {routesQuery.error && (
          <EmptyState
            className="min-h-32"
            title="Unable to load alert routes"
            description="Retry after Core is reachable."
            tone="error"
            action={
              <Button size="sm" variant="outline" onClick={() => void routesQuery.refetch()}>
                Retry
              </Button>
            }
          />
        )}
        {!routesQuery.error && (
          <DataTable
            columns={routeTableColumns}
            data={routes}
            emptyMessage="No explicit alert routes configured; Core will use fallback destinations."
            getRowId={(route, index) => route.id ?? `route-${index}`}
            isLoading={routesQuery.isLoading}
            loadingMessage="Loading alert routes..."
          />
        )}
      </div>

      <div className="space-y-3">
        <div>
          <h2 className="text-sm font-medium">Route Dry Run</h2>
          <p className="text-sm text-neutral-600">
            Evaluate matching and destination decisions without sending notifications.
          </p>
        </div>
        <div className="grid gap-2 md:grid-cols-[180px_160px_160px_minmax(0,1fr)_auto]">
          <Select value={dryRunEventType} onValueChange={setDryRunEventType}>
            <SelectTrigger>
              <span data-slot="select-value">{eventLabel(dryRunEventType)}</span>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="incident_opened">Incident opened</SelectItem>
              <SelectItem value="incident_resolved">Incident resolved</SelectItem>
              <SelectItem value="alert_group_summary">Group summary</SelectItem>
            </SelectContent>
          </Select>
          <Select value={dryRunSeverity} onValueChange={setDryRunSeverity}>
            <SelectTrigger>
              <span data-slot="select-value">Severity: {dryRunSeverity}</span>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="low">Low</SelectItem>
              <SelectItem value="medium">Medium</SelectItem>
              <SelectItem value="high">High</SelectItem>
              <SelectItem value="critical">Critical</SelectItem>
            </SelectContent>
          </Select>
          <Input
            value={dryRunMonitorType}
            onChange={(event) => setDryRunMonitorType(event.target.value)}
            placeholder="Monitor type"
          />
          <Input
            value={dryRunIncidentId}
            onChange={(event) => setDryRunIncidentId(event.target.value)}
            placeholder="Incident ID to load context"
          />
          <Button onClick={runDryRun} disabled={dryRunRoutes.isPending}>
            {dryRunRoutes.isPending ? "Running..." : "Run"}
          </Button>
        </div>
        {dryRunRoutes.isError && <div className="text-sm">Unable to dry-run alert routes.</div>}
        {dryRun && (
          <div className="grid gap-3 lg:grid-cols-2">
            <div className="space-y-2 bg-neutral-50 p-3 text-sm">
              <div className="font-medium">Route evaluations</div>
              {(dryRun.route_evaluations ?? []).length === 0 && (
                <div className="text-neutral-600">No explicit routes evaluated.</div>
              )}
              {(dryRun.route_evaluations ?? []).map((evaluation) => (
                <div key={evaluation.route?.id ?? evaluation.route?.name} className="space-y-1">
                  <div>
                    {evaluation.matched ? "matched" : "skipped"} ·{" "}
                    {evaluation.route?.name ?? "route"}
                    {evaluation.suppressed ? " · suppresses" : ""}
                  </div>
                  <div className="text-neutral-600">
                    {(evaluation.reasons ?? []).join("; ") || "No reasons returned."}
                  </div>
                </div>
              ))}
            </div>
            <div className="space-y-2 bg-neutral-50 p-3 text-sm">
              <div className="font-medium">Destination decisions</div>
              {dryRun.suppressed && (
                <div className="text-neutral-600">
                  Suppressed: {dryRun.suppression_reason ?? "route policy"}
                </div>
              )}
              {(dryRun.destination_decisions ?? []).length === 0 && (
                <div className="text-neutral-600">No destination decisions returned.</div>
              )}
              {(dryRun.destination_decisions ?? []).map((decision, index) => (
                <div key={`${decision.channel_id ?? decision.channel_name ?? "decision"}-${index}`}>
                  {decision.status ?? "unknown"} · {decision.channel_name ?? "destination"} ·{" "}
                  <span className="text-neutral-600">{decision.reason ?? "no reason"}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </section>
  );
};
