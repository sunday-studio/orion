import { DataTable } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { SeverityBadge, toSeverity } from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
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
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import type {
  ApiAlertChannelResponse,
  ApiAlertRouteDryRunResponse,
  ApiAlertRouteRequest,
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
import {
  Bell,
  Clock3,
  FlaskConical,
  GitBranch,
  MoreHorizontal,
  Pencil,
  Power,
  Trash2,
  Webhook,
} from "lucide-react";
import { type FormEvent, useMemo, useState } from "react";
import {
  alertEventOptions,
  boolLabel,
  eventLabel,
  getMutationErrorMessage,
} from "./alert-constants";

const severityOptions = [
  { value: "low", label: "Low" },
  { value: "medium", label: "Medium" },
  { value: "high", label: "High" },
  { value: "critical", label: "Critical" },
  { value: "error", label: "Error" },
] as const;

const groupingOptions = [
  { value: "suppress", label: "Suppress repeats" },
  { value: "delayed_summary", label: "Delayed summary" },
  { value: "none", label: "No grouping" },
] as const;

type RouteFormState = {
  agentIds: string;
  channelIds: string[];
  enabled: boolean;
  eventTypes: string[];
  groupingDelaySeconds: string;
  groupingPolicy: string;
  monitorIds: string;
  monitorTypes: string;
  name: string;
  priority: string;
  severities: string[];
  suppress: boolean;
};

type DryRunFormState = {
  agentId: string;
  eventType: string;
  incidentId: string;
  monitorId: string;
  monitorType: string;
  severity: string;
};

type AlertRulesTabProps = {
  channels: ApiAlertChannelResponse[];
};

const defaultRouteForm = (channels: ApiAlertChannelResponse[]): RouteFormState => ({
  agentIds: "",
  channelIds: channels.find((channel) => channel.id)?.id ? [channels[0].id ?? ""] : [],
  enabled: true,
  eventTypes: alertEventOptions.map((option) => option.value),
  groupingDelaySeconds: "300",
  groupingPolicy: "suppress",
  monitorIds: "",
  monitorTypes: "",
  name: "",
  priority: "100",
  severities: [],
  suppress: false,
});

const defaultDryRunForm: DryRunFormState = {
  agentId: "",
  eventType: "incident_opened",
  incidentId: "",
  monitorId: "",
  monitorType: "",
  severity: "high",
};

const splitList = (value: string) =>
  value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean);

const joinList = (value?: string[]) => (value ?? []).join("\n");

const routeToForm = (
  route: ApiAlertRouteResponse,
  channels: ApiAlertChannelResponse[],
): RouteFormState => ({
  agentIds: joinList(route.agent_ids),
  channelIds: (route.channel_ids ?? []).filter((id) =>
    channels.some((channel) => channel.id === id),
  ),
  enabled: route.enabled ?? true,
  eventTypes: route.event_types?.length
    ? route.event_types
    : alertEventOptions.map((option) => option.value),
  groupingDelaySeconds: String(route.grouping_delay_seconds ?? 300),
  groupingPolicy: route.grouping_policy ?? "suppress",
  monitorIds: joinList(route.monitor_ids),
  monitorTypes: joinList(route.monitor_types),
  name: route.name ?? "",
  priority: String(route.priority ?? 100),
  severities: route.severities ?? [],
  suppress: route.suppress ?? false,
});

const routeToRequest = (route: ApiAlertRouteResponse): ApiAlertRouteRequest => ({
  agent_ids: route.agent_ids ?? [],
  channel_ids: route.suppress ? [] : (route.channel_ids ?? []),
  enabled: route.enabled ?? true,
  event_types: route.event_types ?? alertEventOptions.map((option) => option.value),
  grouping_delay_seconds: route.grouping_delay_seconds ?? 300,
  grouping_policy: route.grouping_policy ?? "suppress",
  monitor_ids: route.monitor_ids ?? [],
  monitor_types: route.monitor_types ?? [],
  name: route.name ?? "",
  priority: route.priority ?? 100,
  severities: route.severities ?? [],
  suppress: route.suppress ?? false,
});

const formToRequest = (form: RouteFormState): ApiAlertRouteRequest => ({
  agent_ids: splitList(form.agentIds),
  channel_ids: form.suppress ? [] : form.channelIds,
  enabled: form.enabled,
  event_types: form.eventTypes,
  grouping_delay_seconds: Math.max(Number.parseInt(form.groupingDelaySeconds, 10) || 300, 1),
  grouping_policy: form.groupingPolicy,
  monitor_ids: splitList(form.monitorIds),
  monitor_types: splitList(form.monitorTypes),
  name: form.name.trim(),
  priority: Math.max(Number.parseInt(form.priority, 10) || 0, 0),
  severities: form.severities,
  suppress: form.suppress,
});

const channelName = (channels: ApiAlertChannelResponse[], id?: string) =>
  channels.find((channel) => channel.id === id)?.name ?? id ?? "unknown";

const formatList = (values?: string[], formatter?: (value: string) => string) => {
  if (!values?.length) return "Any";
  return values.map((value) => formatter?.(value) ?? value).join(", ");
};

const toggleValue = (values: string[], value: string, checked: boolean) => {
  if (checked) return Array.from(new Set([...values, value]));
  return values.filter((item) => item !== value);
};

const FlowNode = ({
  children,
  icon: Icon,
  tone = "neutral",
  title,
}: {
  children: React.ReactNode;
  icon: typeof GitBranch;
  tone?: "neutral" | "warning" | "success";
  title: string;
}) => (
  <div
    className={cn(
      "relative min-h-28 border bg-white p-4 shadow-xs",
      tone === "warning" && "border-amber-300 bg-amber-50",
      tone === "success" && "border-emerald-300 bg-emerald-50",
      tone === "neutral" && "border-neutral-300",
    )}
  >
    <div className="flex items-center gap-2 text-sm font-medium">
      <Icon className="size-4" />
      {title}
    </div>
    <div className="mt-3 text-sm text-neutral-600">{children}</div>
  </div>
);

const Connector = () => <div className="hidden h-px bg-neutral-300 md:block" aria-hidden="true" />;

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
      {Boolean(routesResponse.error) && <div className="text-sm">Unable to load alert rules.</div>}
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

      <Dialog
        open={editorOpen}
        onOpenChange={(open) => (!open ? closeEditor() : setEditorOpen(true))}
      >
        <DialogContent className="max-h-[calc(100vh-3rem)] overflow-y-auto sm:max-w-5xl">
          <form className="space-y-5" onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>{editingRoute ? "Edit alert rule" : "New alert rule"}</DialogTitle>
              <DialogDescription>
                Connect trigger conditions to suppression, grouping, and webhook delivery.
              </DialogDescription>
            </DialogHeader>

            <div className="grid gap-4 md:grid-cols-[1fr_1.5rem_1fr_1.5rem_1fr]">
              <FlowNode icon={GitBranch} title="Trigger">
                <div className="space-y-3">
                  <label className="block space-y-1">
                    <span className="text-xs font-medium text-neutral-700">Name</span>
                    <Input
                      value={form.name}
                      onChange={(event) => setForm({ ...form, name: event.target.value })}
                    />
                  </label>
                  <label className="block space-y-1">
                    <span className="text-xs font-medium text-neutral-700">Priority</span>
                    <Input
                      min={0}
                      type="number"
                      value={form.priority}
                      onChange={(event) => setForm({ ...form, priority: event.target.value })}
                    />
                  </label>
                  <label className="flex items-center gap-2">
                    <Checkbox
                      checked={form.enabled}
                      onCheckedChange={(checked) => setForm({ ...form, enabled: Boolean(checked) })}
                    />
                    <span>Enabled</span>
                  </label>
                </div>
              </FlowNode>
              <Connector />
              <FlowNode icon={Bell} title="Match">
                <div className="space-y-4">
                  <div className="space-y-2">
                    <div className="text-xs font-medium text-neutral-700">Events</div>
                    {alertEventOptions.map((option) => (
                      <label key={option.value} className="flex items-center gap-2">
                        <Checkbox
                          checked={form.eventTypes.includes(option.value)}
                          onCheckedChange={(checked) =>
                            setForm({
                              ...form,
                              eventTypes: toggleValue(
                                form.eventTypes,
                                option.value,
                                Boolean(checked),
                              ),
                            })
                          }
                        />
                        <span>{option.label}</span>
                      </label>
                    ))}
                  </div>
                  <div className="space-y-2">
                    <div className="text-xs font-medium text-neutral-700">Severity</div>
                    <div className="grid grid-cols-2 gap-2">
                      {severityOptions.map((option) => (
                        <label key={option.value} className="flex items-center gap-2">
                          <Checkbox
                            checked={form.severities.includes(option.value)}
                            onCheckedChange={(checked) =>
                              setForm({
                                ...form,
                                severities: toggleValue(
                                  form.severities,
                                  option.value,
                                  Boolean(checked),
                                ),
                              })
                            }
                          />
                          <span>{option.label}</span>
                        </label>
                      ))}
                    </div>
                  </div>
                </div>
              </FlowNode>
              <Connector />
              <FlowNode icon={Webhook} tone={form.suppress ? "warning" : "success"} title="Action">
                <div className="space-y-3">
                  <label className="flex items-center gap-2">
                    <Checkbox
                      checked={form.suppress}
                      onCheckedChange={(checked) =>
                        setForm({ ...form, suppress: Boolean(checked), channelIds: [] })
                      }
                    />
                    <span>Suppress matching alerts</span>
                  </label>
                  {!form.suppress && (
                    <div className="space-y-2">
                      <div className="text-xs font-medium text-neutral-700">
                        Webhook destinations
                      </div>
                      {webhookChannels.length === 0 ? (
                        <div className="text-sm text-red-700">
                          Create a webhook destination first.
                        </div>
                      ) : (
                        webhookChannels.map((channel) => (
                          <label key={channel.id} className="flex items-center gap-2">
                            <Checkbox
                              checked={form.channelIds.includes(channel.id ?? "")}
                              onCheckedChange={(checked) =>
                                setForm({
                                  ...form,
                                  channelIds: toggleValue(
                                    form.channelIds,
                                    channel.id ?? "",
                                    Boolean(checked),
                                  ),
                                })
                              }
                            />
                            <span>{channel.name ?? channel.id}</span>
                          </label>
                        ))
                      )}
                    </div>
                  )}
                </div>
              </FlowNode>
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <FlowNode icon={Clock3} title="Grouping">
                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="block space-y-1">
                    <span className="text-xs font-medium text-neutral-700">Policy</span>
                    <Select
                      value={form.groupingPolicy}
                      onValueChange={(value) => setForm({ ...form, groupingPolicy: value })}
                    >
                      <SelectTrigger className="w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {groupingOptions.map((option) => (
                          <SelectItem key={option.value} value={option.value}>
                            {option.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </label>
                  <label className="block space-y-1">
                    <span className="text-xs font-medium text-neutral-700">Delay seconds</span>
                    <Input
                      min={1}
                      type="number"
                      value={form.groupingDelaySeconds}
                      onChange={(event) =>
                        setForm({ ...form, groupingDelaySeconds: event.target.value })
                      }
                    />
                  </label>
                </div>
              </FlowNode>
              <FlowNode icon={GitBranch} title="Filters">
                <div className="grid gap-3 sm:grid-cols-3">
                  <label className="block space-y-1">
                    <span className="text-xs font-medium text-neutral-700">Agent IDs</span>
                    <Textarea
                      className="min-h-20 text-sm"
                      value={form.agentIds}
                      onChange={(event) => setForm({ ...form, agentIds: event.target.value })}
                    />
                  </label>
                  <label className="block space-y-1">
                    <span className="text-xs font-medium text-neutral-700">Monitor IDs</span>
                    <Textarea
                      className="min-h-20 text-sm"
                      value={form.monitorIds}
                      onChange={(event) => setForm({ ...form, monitorIds: event.target.value })}
                    />
                  </label>
                  <label className="block space-y-1">
                    <span className="text-xs font-medium text-neutral-700">Monitor types</span>
                    <Textarea
                      className="min-h-20 text-sm"
                      value={form.monitorTypes}
                      onChange={(event) => setForm({ ...form, monitorTypes: event.target.value })}
                    />
                  </label>
                </div>
              </FlowNode>
            </div>

            <div className="border border-neutral-300 bg-neutral-50 p-4">
              <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
                <div className="flex items-center gap-2 text-sm font-medium">
                  <FlaskConical className="size-4" />
                  Dry Run
                </div>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={dryRun.isPending}
                  onClick={handleDryRun}
                >
                  {dryRun.isPending ? "Running..." : "Run"}
                </Button>
              </div>
              <div className="grid gap-3 md:grid-cols-3">
                <Select
                  value={dryRunForm.eventType}
                  onValueChange={(value) => setDryRunForm({ ...dryRunForm, eventType: value })}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {alertEventOptions.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Select
                  value={dryRunForm.severity}
                  onValueChange={(value) => setDryRunForm({ ...dryRunForm, severity: value })}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {severityOptions.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Input
                  placeholder="incident id"
                  value={dryRunForm.incidentId}
                  onChange={(event) =>
                    setDryRunForm({ ...dryRunForm, incidentId: event.target.value })
                  }
                />
                <Input
                  placeholder="agent id"
                  value={dryRunForm.agentId}
                  onChange={(event) =>
                    setDryRunForm({ ...dryRunForm, agentId: event.target.value })
                  }
                />
                <Input
                  placeholder="monitor id"
                  value={dryRunForm.monitorId}
                  onChange={(event) =>
                    setDryRunForm({ ...dryRunForm, monitorId: event.target.value })
                  }
                />
                <Input
                  placeholder="monitor type"
                  value={dryRunForm.monitorType}
                  onChange={(event) =>
                    setDryRunForm({ ...dryRunForm, monitorType: event.target.value })
                  }
                />
              </div>
              {dryRun.error && (
                <div className="mt-3 text-sm text-red-700">
                  {getMutationErrorMessage(dryRun.error, "Dry run failed.")}
                </div>
              )}
              {dryRunResult && (
                <div className="mt-4 grid gap-3 md:grid-cols-2">
                  <div className="border border-neutral-300 bg-white p-3 text-sm">
                    <div className="font-medium">Route evaluations</div>
                    {dryRunResult.route_evaluations?.length ? (
                      <ul className="mt-2 space-y-2">
                        {dryRunResult.route_evaluations.map((evaluation, index) => (
                          <li key={`${evaluation.route?.id ?? "route"}-${index}`}>
                            <span className="font-medium">
                              {evaluation.route?.name ?? "Unnamed"}
                            </span>
                            {evaluation.matched ? " matched" : " did not match"}
                            {evaluation.reasons?.length ? `: ${evaluation.reasons.join(", ")}` : ""}
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <EmptyState title="No route evaluations returned." />
                    )}
                  </div>
                  <div className="border border-neutral-300 bg-white p-3 text-sm">
                    <div className="font-medium">Destination decisions</div>
                    {dryRunResult.destination_decisions?.length ? (
                      <ul className="mt-2 space-y-2">
                        {dryRunResult.destination_decisions.map((decision, index) => (
                          <li key={`${decision.channel_id ?? "decision"}-${index}`}>
                            <span className="font-medium">
                              {decision.channel_name ?? decision.channel_id ?? "Destination"}
                            </span>
                            {` ${decision.status ?? "unknown"}`}
                            {decision.reason ? `: ${decision.reason}` : ""}
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <div className="mt-2 text-neutral-600">
                        {dryRunResult.suppressed
                          ? (dryRunResult.suppression_reason ?? "Suppressed")
                          : "No destinations selected."}
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={closeEditor}>
                Cancel
              </Button>
              <Button type="submit" disabled={!canSubmit}>
                {isSaving ? "Saving..." : editingRoute ? "Save rule" : "Create rule"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

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
