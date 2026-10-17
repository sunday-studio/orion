import { useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";
import { Link } from "react-router-dom";
import {
  Button,
  ConfirmDialog,
  DataTable,
  DataTableLink,
  PageHeader,
  StatusBadge,
  useToast,
} from "@/design-system";
import {
  type ApiIncidentCandidate,
  getGetIncidentsCandidatesQueryKey,
  useDismissIncidentCandidate,
  useGetIncidentsCandidates,
} from "@/orion-sdk";

export const IncidentTriagePage = () => {
  const queryClient = useQueryClient();
  const toast = useToast();
  const candidatesQuery = useGetIncidentsCandidates();
  const [dismissing, setDismissing] = useState<ApiIncidentCandidate | null>(null);
  const dismiss = useDismissIncidentCandidate({
    mutation: {
      onSuccess: () => {
        void queryClient.invalidateQueries({ queryKey: getGetIncidentsCandidatesQueryKey() });
        toast.add({
          title: "Candidate dismissed",
          description: "It returns if the monitor fails again.",
        });
      },
      onError: () => toast.add({ title: "Unable to dismiss candidate" }),
    },
  });
  const candidates = candidatesQuery.data?.candidates ?? [];

  const columns: ColumnDef<ApiIncidentCandidate>[] = [
    {
      id: "monitor",
      header: "Monitor",
      cell: ({ row }) =>
        row.original.monitor_id ? (
          <DataTableLink to={`/monitors/${row.original.monitor_id}`}>
            {row.original.monitor_name ?? row.original.monitor_id}
          </DataTableLink>
        ) : (
          (row.original.monitor_name ?? "-")
        ),
    },
    {
      id: "server",
      header: "Server",
      cell: ({ row }) =>
        row.original.agent_id ? (
          <DataTableLink to={`/servers/${row.original.agent_id}`}>
            {row.original.agent_name ?? row.original.agent_id}
          </DataTableLink>
        ) : (
          "Core"
        ),
    },
    {
      accessorKey: "monitor_type",
      header: "Type",
      cell: ({ row }) => row.original.monitor_type ?? "-",
    },
    {
      id: "health",
      header: "Health",
      cell: ({ row }) => <StatusBadge status={row.original.health} />,
    },
    {
      id: "severity",
      header: "Severity",
      cell: ({ row }) => <StatusBadge status={row.original.severity} />,
    },
    {
      accessorKey: "down_count",
      header: "Failed of last 5",
      cell: ({ row }) =>
        row.original.issue_type === "stale_data" ? "stale" : `${row.original.down_count ?? 0} / 5`,
    },
    {
      id: "actions",
      header: "",
      cell: ({ row }) => (
        <Button size="sm" variant="secondary" onClick={() => setDismissing(row.original)}>
          Dismiss
        </Button>
      ),
    },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <PageHeader
          title="Triage"
          description="Failing monitors that are not yet incidents. Dismiss the noise; they return if the monitor fails again."
        />
        <Link className="text-[13px] text-brand hover:underline" to="/incidents">
          All incidents
        </Link>
      </div>
      <DataTable
        columns={columns}
        data={candidates}
        getRowId={(candidate, index) => candidate.monitor_id ?? `candidate-${index}`}
        isLoading={candidatesQuery.isLoading}
        error={
          candidatesQuery.isError
            ? {
                message: "Unable to load candidates",
                onRetry: () => void candidatesQuery.refetch(),
              }
            : undefined
        }
        emptyMessage="Nothing to triage. No failing monitors need attention."
      />
      <ConfirmDialog
        open={dismissing !== null}
        onOpenChange={(open) => {
          if (!open) setDismissing(null);
        }}
        title="Dismiss candidate?"
        description={`${dismissing?.monitor_name ?? "This monitor"} will be hidden from triage until it recovers and fails again.`}
        confirmLabel="Dismiss"
        tone="default"
        onConfirm={() => {
          if (dismissing?.monitor_id) dismiss.mutate({ id: dismissing.monitor_id });
        }}
      />
    </div>
  );
};
