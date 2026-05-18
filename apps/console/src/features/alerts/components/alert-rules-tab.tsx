import { DataTable } from "@/components/data-table";
import { SeverityBadge, toSeverity } from "@/components/status-badges";
import type { ApiAlertRuleResponse } from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import { useMemo } from "react";
import { boolLabel } from "./alert-constants";

const getRuleColumns = (webhookChannelNames: Set<string>): ColumnDef<ApiAlertRuleResponse>[] => [
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
    header: "Channels",
    cell: ({ row }) =>
      (row.original.target_channels ?? [])
        .filter((channel) => webhookChannelNames.has(channel))
        .join(", ") || "none",
  },
];

type AlertRulesTabProps = {
  error: unknown;
  isLoading: boolean;
  rules: ApiAlertRuleResponse[];
  webhookChannelNames: Set<string>;
};

export const AlertRulesTab = ({
  error,
  isLoading,
  rules,
  webhookChannelNames,
}: AlertRulesTabProps) => {
  const columns = useMemo(() => getRuleColumns(webhookChannelNames), [webhookChannelNames]);

  return (
    <section className="space-y-3">
      <div>
        <h2 className="text-sm font-medium">Rules</h2>
        <p className="text-sm text-neutral-600">
          These are the effective alert rules Core applies during incident reconciliation.
        </p>
      </div>
      {Boolean(error) && <div className="text-sm">Unable to load alert rules.</div>}
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
    </section>
  );
};
