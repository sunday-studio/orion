import { DataTable } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { SeverityBadge, toSeverity } from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type {
  ApiAlertChannelResponse,
  ApiAlertRouteDryRunResponse,
  ApiAlertRouteResponse,
} from "@/orion-sdk";
import {
  useCreateAlertRoute,
  useDeleteAlertRoute,
  useDryRunAlertRoutes,
  useGetAlertRoutes,
  useUpdateAlertRoute,
} from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import { FlaskConical, MoreHorizontal, Pencil, Power, Trash2 } from "lucide-react";
import { type FormEvent, useMemo, useState } from "react";
import { boolLabel, eventLabel, getMutationErrorMessage } from "./alert-constants";
import { AlertRuleEditorDialog } from "./alert-rule-editor-dialog";
import {
  channelName,
  defaultDryRunForm,
  defaultRouteForm,
  formToRequest,
  formatList,
  type DryRunFormState,
  routeToForm,
  routeToRequest,
  type RouteFormState,
} from "./alert-rules-utils";

type AlertRulesTabProps = {
  channels: ApiAlertChannelResponse[];
};

export const AlertRulesTab = ({ channels }: AlertRulesTabProps) => {
  const routesResponse = useGetAlertRoutes();
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingRoute, setEditingRoute] = useState<ApiAlertRouteResponse | null>(null);
  const [deletingRoute, setDeletingRoute] = useState<ApiAlertRouteResponse | null>(null);
  const [form, setForm] = useState<RouteFormState>(() => defaultRouteForm(channels));
  const [dryRunForm, setDryRunForm] = useState<DryRunFormState>(defaultDryRunForm);
  const [dryRunResult, setDryRunResult] = useState<ApiAlertRouteDryRunResponse | null>(null);

  const webhookChannels = useMemo(
    () => channels.filter((channel) => channel.type === "webhook" && channel.id),
    [channels],
  );
  const routes = routesResponse.data?.routes ?? [];

  const closeEditor = () => {
    setEditorOpen(false);
    setEditingRoute(null);
    setForm(defaultRouteForm(webhookChannels));
    setDryRunResult(null);
  };

  const createRoute = useCreateAlertRoute({
    mutation: {
      onSuccess: () => {
        closeEditor();
        void routesResponse.refetch();
      },
    },
  });
  const updateRoute = useUpdateAlertRoute({
    mutation: {
      onSuccess: () => {
        closeEditor();
        void routesResponse.refetch();
      },
    },
  });
  const deleteRoute = useDeleteAlertRoute({
    mutation: {
      onSuccess: () => {
        setDeletingRoute(null);
        void routesResponse.refetch();
      },
    },
  });
  const dryRun = useDryRunAlertRoutes({
    mutation: {
      onSuccess: (data) => {
        setDryRunResult(data.dry_run ?? null);
      },
    },
  });

  const isSaving = createRoute.isPending || updateRoute.isPending;
  const mutationError = getMutationErrorMessage(
    createRoute.error ?? updateRoute.error ?? deleteRoute.error,
    "Unable to save alert rule.",
  );
  const canSubmit =
    form.name.trim().length > 0 &&
    form.eventTypes.length > 0 &&
    (form.suppress || form.channelIds.length > 0) &&
    !isSaving;

  const openCreateEditor = () => {
    setEditingRoute(null);
    setForm(defaultRouteForm(webhookChannels));
    setDryRunResult(null);
    setEditorOpen(true);
  };
  const openEditEditor = (route: ApiAlertRouteResponse) => {
    setEditingRoute(route);
    setForm(routeToForm(route, webhookChannels));
    setDryRunResult(null);
    setEditorOpen(true);
  };
  const openDryRunEditor = (route: ApiAlertRouteResponse) => {
    openEditEditor(route);
    setDryRunResult(null);
  };
  const setEnabled = (route: ApiAlertRouteResponse, enabled: boolean) => {
    if (!route.id || updateRoute.isPending) return;
    updateRoute.mutate({ id: route.id, data: { ...routeToRequest(route), enabled } });
  };
  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmit) return;

    const data = formToRequest(form);
    if (editingRoute?.id) {
      updateRoute.mutate({ id: editingRoute.id, data });
      return;
    }
    createRoute.mutate({ data });
  };
  const handleDryRun = () => {
    dryRun.mutate({
      data: {
        agent_id: dryRunForm.agentId.trim() || undefined,
        event_type: dryRunForm.eventType,
        incident_id: dryRunForm.incidentId.trim() || undefined,
        monitor_id: dryRunForm.monitorId.trim() || undefined,
        monitor_type: dryRunForm.monitorType.trim() || undefined,
        severity: dryRunForm.severity,
      },
    });
  };

  const columns: ColumnDef<ApiAlertRouteResponse>[] = [
    {
      accessorKey: "name",
      header: "Name",
      cell: ({ row }) => <span className="font-medium">{row.original.name ?? "unnamed"}</span>,
    },
    {
      accessorKey: "enabled",
      header: "Enabled",
      cell: ({ row }) => boolLabel(row.original.enabled),
    },
    {
      accessorKey: "priority",
      header: "Priority",
      cell: ({ row }) => row.original.priority ?? 0,
    },
    {
      accessorKey: "severities",
      header: "Severity",
      cell: ({ row }) =>
        row.original.severities?.length ? (
          <div className="flex flex-wrap gap-1">
            {row.original.severities.map((severity) => (
              <SeverityBadge key={severity} value={toSeverity(severity)} />
            ))}
          </div>
        ) : (
          "Any"
        ),
    },
    {
      accessorKey: "event_types",
      header: "Events",
      cell: ({ row }) => (
        <div className="max-w-72 truncate text-neutral-600">
          {formatList(row.original.event_types, eventLabel)}
        </div>
      ),
    },
    {
      accessorKey: "channel_ids",
      header: "Destinations",
      cell: ({ row }) =>
        row.original.suppress ? (
          <span className="text-amber-700">Suppressed</span>
        ) : (
          <div className="max-w-80 truncate text-neutral-600">
            {formatList(row.original.channel_ids, (id) => channelName(webhookChannels, id))}
          </div>
        ),
    },
    {
      id: "actions",
      header: "",
      cell: ({ row }) => (
        <DropdownMenu>
          <DropdownMenuTrigger
            aria-label={`Open actions for ${row.original.name ?? "alert rule"}`}
            className="ml-auto flex size-6 items-center justify-center hover:bg-accent focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            <MoreHorizontal className="size-4" />
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuItem onClick={() => openDryRunEditor(row.original)}>
              <FlaskConical className="size-4" />
              Dry run
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => openEditEditor(row.original)}>
              <Pencil className="size-4" />
              Edit
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={!row.original.id || updateRoute.isPending}
              onClick={() => setEnabled(row.original, !(row.original.enabled ?? true))}
            >
              <Power className="size-4" />
              {(row.original.enabled ?? true) ? "Disable" : "Enable"}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => setDeletingRoute(row.original)}>
              <Trash2 className="size-4" />
              Delete
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-medium">Alert Rules</h2>
          <p className="text-sm text-neutral-600">
            Priority-ordered rules that match incidents, optionally suppress noise, and target
            webhook destinations.
          </p>
        </div>
        <Button size="sm" onClick={openCreateEditor}>
          New rule
        </Button>
      </div>

      {mutationError && <div className="text-sm text-red-700">{mutationError}</div>}
      {Boolean(routesResponse.error) && (
        <EmptyState
          className="min-h-40"
          title="Unable to load alert rules"
          description="Retry after Core is reachable."
          tone="error"
          action={
            <Button size="sm" variant="outline" onClick={() => void routesResponse.refetch()}>
              Retry
            </Button>
          }
        />
      )}
      {!routesResponse.error && (
        <DataTable
          columns={columns}
          data={routes}
          emptyMessage="No alert rules configured."
          getRowId={(route, index) => route.id ?? route.name ?? `route-${index}`}
          isLoading={routesResponse.isLoading}
          loadingMessage="Loading alert rules..."
        />
      )}

      <AlertRuleEditorDialog
        canSubmit={Boolean(canSubmit)}
        closeEditor={closeEditor}
        dryRun={dryRun}
        dryRunForm={dryRunForm}
        dryRunResult={dryRunResult}
        editingRoute={editingRoute}
        editorOpen={editorOpen}
        form={form}
        handleDryRun={handleDryRun}
        handleSubmit={handleSubmit}
        isSaving={isSaving}
        setDryRunForm={setDryRunForm}
        setEditorOpen={setEditorOpen}
        setForm={setForm}
        webhookChannels={webhookChannels}
      />

      <Dialog
        open={Boolean(deletingRoute)}
        onOpenChange={(open) => !open && setDeletingRoute(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete alert rule</DialogTitle>
            <DialogDescription>
              Delete {deletingRoute?.name ?? "this alert rule"} from future alert routing.
            </DialogDescription>
          </DialogHeader>
          {deleteRoute.error && (
            <div className="text-sm text-red-700">
              {getMutationErrorMessage(deleteRoute.error, "Unable to delete alert rule.")}
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeletingRoute(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={!deletingRoute?.id || deleteRoute.isPending}
              onClick={() => {
                if (!deletingRoute?.id) return;
                deleteRoute.mutate({ id: deletingRoute.id });
              }}
            >
              {deleteRoute.isPending ? "Deleting..." : "Delete rule"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
};
