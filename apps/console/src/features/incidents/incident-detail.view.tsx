import { DataTable } from "@/components/data-table";
import { PageBreadcrumbs } from "@/components/page-breadcrumbs";
import {
  NotificationBadge,
  SeverityBadge,
  StatusBadge,
  toNotificationStatus,
  toSeverity,
  toStatus,
} from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { TabCount, Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { ReportInspectionDrawer } from "@/features/report-inspection/report-inspection-drawer";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import {
  type ApiAlertDeliveryResponse,
  type ApiIncidentNextActionResponse,
  type ApiIncidentResponse,
  type ApiIncidentTimelineItemResponse,
  type ApiMonitorReportResponse,
  getGetIncidentQueryKey,
  getGetIncidentTimelineQueryKey,
  useAcknowledgeIncident,
  useCoverIncident,
  useGetIncident,
  useReopenIncident,
  useResolveIncident,
} from "@/orion-sdk";
import { useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import {
  BellRingIcon,
  CheckIcon,
  CircleCheckIcon,
  RotateCcwIcon,
  ShieldCheckIcon,
  WrenchIcon,
} from "lucide-react";
import { type FormEvent, type ReactNode, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";

const DetailItem = ({ label, value }: { label: string; value: ReactNode }) => (
  <div>
    <div className="text-sm text-neutral-600">{label}</div>
    <div className="text-sm font-medium">{value}</div>
  </div>
);

const DetailGroup = ({ title, children }: { title: string; children: ReactNode }) => (
  <div className="space-y-3 bg-neutral-50 px-3 py-3">
    <h2 className="text-sm font-medium">{title}</h2>
    <div className="space-y-3">{children}</div>
  </div>
);

const durationLabel = (incident: ApiIncidentResponse) => {
  const start = incident.opened_at ? new Date(incident.opened_at).getTime() : undefined;
  const end = incident.resolved_at ? new Date(incident.resolved_at).getTime() : Date.now();
  if (!start || Number.isNaN(start) || Number.isNaN(end)) return "—";
  const seconds = Math.max(0, Math.floor((end - start) / 1000));
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
};

const coverageUntilInputValue = () => {
  const value = new Date(Date.now() + 60 * 60 * 1000);
  const offset = value.getTimezoneOffset() * 60000;
  return new Date(value.getTime() - offset).toISOString().slice(0, 16);
};

const coverageUntilPayload = (value: string) => {
  if (!value) return undefined;
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? undefined : parsed.toISOString();
};

type IncidentComponentImpact = NonNullable<ApiIncidentResponse["impacted_components"]>[number];

const componentLabel = (component: IncidentComponentImpact) =>
  component.component_name || component.component_id || "Unnamed component";

const componentImpactLabel = (component: IncidentComponentImpact) =>
  component.impact || component.status || "";

const ComponentImpactList = ({ components }: { components: IncidentComponentImpact[] }) => {
  if (components.length === 0) {
    return <span className="text-neutral-500">No components</span>;
  }

  return (
    <div className="space-y-1">
      {components.map((component, index) => {
        const impact = componentImpactLabel(component);
        return (
          <div
            key={`${component.component_id ?? component.component_name ?? "component"}-${index}`}
            className="min-w-0"
          >
            <div className="truncate">{componentLabel(component)}</div>
            {impact && <div className="truncate text-xs text-neutral-500">{impact}</div>}
          </div>
        );
      })}
    </div>
  );
};

const actionIcon = (actionType?: string) => {
  switch (actionType) {
    case "acknowledge_incident":
      return <CheckIcon />;
    case "cover_incident":
      return <ShieldCheckIcon />;
    case "resolve_incident":
      return <CircleCheckIcon />;
    case "reopen_incident":
      return <RotateCcwIcon />;
    case "review_monitor_tuning":
      return <WrenchIcon />;
    case "review_failed_notifications":
      return <BellRingIcon />;
    default:
      return <CheckIcon />;
  }
};

const nextActionHref = (
  action: ApiIncidentNextActionResponse,
  incident: ApiIncidentResponse,
) => {
  if (action.target_kind === "monitor" && action.target_id) {
    const params = new URLSearchParams();
    if (action.target_tab) params.set("tab", action.target_tab);
    if (incident.id) params.set("incident", incident.id);
    const query = params.toString();
    return `/monitors/${action.target_id}${query ? `?${query}` : ""}`;
  }
  if (action.target_kind === "alert_deliveries") {
    const params = new URLSearchParams();
    params.set("tab", action.target_tab || "logs");
    params.set("incident", action.target_id || incident.id || "");
    if (action.filter_status) params.set("status", action.filter_status);
    return `/alerts?${params.toString()}`;
  }
  return undefined;
};

const isIncidentMutationAction = (actionType?: string) =>
  actionType === "acknowledge_incident" ||
  actionType === "cover_incident" ||
  actionType === "resolve_incident" ||
  actionType === "reopen_incident";

type IncidentNextActionPanelProps = {
  actions: ApiIncidentNextActionResponse[];
  actionPending: boolean;
  incident: ApiIncidentResponse;
  onAction: (action: ApiIncidentNextActionResponse) => void;
};

const IncidentNextActionPanel = ({
  actions,
  actionPending,
  incident,
  onAction,
}: IncidentNextActionPanelProps) => {
  if (actions.length === 0) {
    return (
      <section className="space-y-2 bg-neutral-50 px-3 py-3">
        <h2 className="text-sm font-medium">Next Actions</h2>
        <p className="text-sm text-neutral-600">No operator action is currently suggested.</p>
      </section>
    );
  }

  return (
    <section className="space-y-3 bg-neutral-50 px-3 py-3">
      <h2 className="text-sm font-medium">Next Actions</h2>
      <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
        {actions.map((action) => {
          const href = nextActionHref(action, incident);
          return (
            <div
              key={action.id ?? action.action_type ?? action.label}
              className="flex min-w-0 flex-col justify-between gap-3 border border-neutral-200 bg-white px-3 py-3"
            >
              <div className="min-w-0 space-y-1">
                <div className="flex items-center gap-2 text-sm font-medium">
                  {actionIcon(action.action_type)}
                  <span className="truncate">{action.label ?? "Review action"}</span>
                </div>
                <p className="text-sm text-neutral-600">
                  {action.description ?? "Review the incident context before continuing."}
                </p>
              </div>
              {href ? (
                <Link
                  className="inline-flex h-8 items-center justify-center gap-2 border border-input bg-background px-3 text-xs font-medium shadow-xs hover:bg-accent hover:text-accent-foreground"
                  to={href}
                >
                  {actionIcon(action.action_type)}
                  Open
                </Link>
              ) : (
                <Button
                  disabled={actionPending || !isIncidentMutationAction(action.action_type)}
                  onClick={() => onAction(action)}
                  size="sm"
                  variant={action.action_type === "resolve_incident" ? "default" : "outline"}
                >
                  {actionIcon(action.action_type)}
                  {action.label ?? "Apply"}
                </Button>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
};

type MonitorPayload = Record<string, unknown>;

const parsePayload = (payload?: string): MonitorPayload => {
  if (!payload) return {};
  try {
    const parsed = JSON.parse(payload);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
  } catch {
    return {};
  }
};

const readPayloadValue = (payload: MonitorPayload, keys: string[]) => {
  for (const key of keys) {
    const value = payload[key];
    if (typeof value === "string" && value.trim() !== "") return value;
    if (typeof value === "number") return String(value);
    if (typeof value === "boolean") return value ? "true" : "false";
  }
  return "—";
};

const reportTimestamp = (report?: ApiMonitorReportResponse) =>
  report?.created_at ?? report?.collected_at;

const reportReason = (report?: ApiMonitorReportResponse) => {
  if (!report) return "No linked monitor report.";
  const payload = parsePayload(report.payload);
  return readPayloadValue(payload, [
    "failure_reason",
    "error",
    "message",
    "summary",
    "status",
    "status_code",
  ]);
};

const reportSortTime = (report: ApiMonitorReportResponse) => {
  const timestamp = reportTimestamp(report);
  const value = timestamp ? new Date(timestamp).getTime() : 0;
  return Number.isNaN(value) ? 0 : value;
};

const detailTabs = ["timeline", "notifications", "monitor-reports"] as const;
type DetailTab = (typeof detailTabs)[number];

const isDetailTab = (value: string | null): value is DetailTab =>
  detailTabs.includes(value as DetailTab);

const timelineColumns = (
  reportsByID: Map<string, ApiMonitorReportResponse>,
): ColumnDef<ApiIncidentTimelineItemResponse>[] => [
  {
    accessorKey: "created_at",
    header: "Time",
    cell: ({ row }) => formatDate(row.original.created_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "type",
    header: "Type",
    cell: ({ row }) => row.original.type ?? "unknown",
  },
  {
    accessorKey: "source",
    header: "Source",
    cell: ({ row }) => row.original.source ?? "unknown",
  },
  {
    accessorKey: "message",
    header: "Message",
    cell: ({ row }) => (
      <div className="max-w-[22rem] truncate text-neutral-600">{row.original.message ?? "—"}</div>
    ),
  },
  {
    id: "evidence",
    header: "Evidence",
    cell: ({ row }) => {
      const report = row.original.monitor_report_id
        ? reportsByID.get(row.original.monitor_report_id)
        : undefined;
      return (
        <div className="max-w-[22rem] truncate text-neutral-600">
          {row.original.evidence ?? reportReason(report)}
        </div>
      );
    },
  },
];

const notificationColumns: ColumnDef<ApiAlertDeliveryResponse>[] = [
  {
    accessorKey: "created_at",
    header: "Time",
    cell: ({ row }) => formatDate(row.original.created_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "channel",
    header: "Channel",
    cell: ({ row }) => row.original.channel ?? "none",
  },
  {
    accessorKey: "event_type",
    header: "Event",
    cell: ({ row }) => row.original.event_type ?? "unknown",
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ row }) => <NotificationBadge value={toNotificationStatus(row.original.status)} />,
  },
  {
    accessorKey: "error",
    header: "Error",
    cell: ({ row }) => (
      <div className="max-w-[24rem] truncate text-neutral-600">{row.original.error ?? "—"}</div>
    ),
  },
];

const monitorReportColumns: ColumnDef<ApiMonitorReportResponse>[] = [
  {
    accessorKey: "created_at",
    header: "Time",
    cell: ({ row }) =>
      formatDate(row.original.created_at ?? row.original.collected_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "health",
    header: "Health",
    cell: ({ row }) => <StatusBadge value={toStatus(row.original.health)} />,
  },
  {
    accessorKey: "id",
    header: "Report ID",
    cell: ({ row }) => (
      <div className="max-w-[24rem] truncate text-neutral-600">{row.original.id ?? "—"}</div>
    ),
  },
];

type CoverIncidentDialogProps = {
  open: boolean;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (payload: { covered_until?: string; note?: string }) => void;
};

const CoverIncidentDialog = ({
  open,
  pending,
  onOpenChange,
  onSubmit,
}: CoverIncidentDialogProps) => {
  const [coveredUntil, setCoveredUntil] = useState(coverageUntilInputValue);
  const [note, setNote] = useState("");

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onSubmit({
      covered_until: coverageUntilPayload(coveredUntil),
      note: note.trim() || undefined,
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={handleSubmit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Cover incident</DialogTitle>
          </DialogHeader>
          <label className="block space-y-1">
            <span className="text-sm font-medium">covered until</span>
            <Input
              type="datetime-local"
              value={coveredUntil}
              onChange={(event) => setCoveredUntil(event.target.value)}
            />
          </label>
          <label className="block space-y-1">
            <span className="text-sm font-medium">note</span>
            <Textarea value={note} onChange={(event) => setNote(event.target.value)} />
          </label>
          <DialogFooter showCloseButton>
            <Button type="submit" disabled={pending}>
              <ShieldCheckIcon />
              Cover
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};

const EvidenceReportGroup = ({
  title,
  report,
  reason,
  onInspect,
}: {
  title: string;
  report?: ApiMonitorReportResponse;
  reason: string;
  onInspect: () => void;
}) => (
  <DetailGroup title={title}>
    <DetailItem
      label="result"
      value={
        <span className="inline-flex items-center gap-2">
          <StatusBadge value={toStatus(report?.health)} />
          <span>{formatDate(reportTimestamp(report), DATE_TIME_FORMAT)}</span>
        </span>
      }
    />
    <DetailItem label="reason" value={reason} />
    <Button type="button" variant="outline" size="sm" disabled={!report} onClick={onInspect}>
      Inspect report
    </Button>
  </DetailGroup>
);

export const IncidentDetailPage = () => {
  const { incidentId = "" } = useParams();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const refreshIncident = () => {
    void queryClient.invalidateQueries({ queryKey: getGetIncidentQueryKey(incidentId) });
    void queryClient.invalidateQueries({ queryKey: getGetIncidentTimelineQueryKey(incidentId) });
    void queryClient.invalidateQueries({ queryKey: ["/v1/incidents"] });
  };
  const incidentResponse = useGetIncident(incidentId);
  const acknowledgeIncident = useAcknowledgeIncident({ mutation: { onSuccess: refreshIncident } });
  const resolveIncident = useResolveIncident({ mutation: { onSuccess: refreshIncident } });
  const coverIncident = useCoverIncident({ mutation: { onSuccess: refreshIncident } });
  const reopenIncident = useReopenIncident({ mutation: { onSuccess: refreshIncident } });
  const incident = incidentResponse.data?.incident;
  const impactedComponents = incident?.impacted_components ?? [];
  const evidence = incidentResponse.data?.evidence;
  const nextActions = incidentResponse.data?.next_actions ?? [];
  const timeline = incidentResponse.data?.timeline ?? [];
  const alertDeliveries = incidentResponse.data?.alert_deliveries ?? [];
  const monitorReports = incidentResponse.data?.monitor_reports ?? [];
  const relatedIncidents = incidentResponse.data?.related_incidents ?? [];
  const [selectedMonitorReport, setSelectedMonitorReport] = useState<ApiMonitorReportResponse>();
  const [coverDialogOpen, setCoverDialogOpen] = useState(false);
  const sortedMonitorReports = [...monitorReports].sort(
    (a, b) => reportSortTime(a) - reportSortTime(b),
  );
  const reportsByID = new Map(
    monitorReports.flatMap((report) => (report.id ? [[report.id, report] as const] : [])),
  );
  const triggeringReport =
    evidence?.triggering_report ??
    sortedMonitorReports.find((report) => report.health && report.health !== "up") ??
    sortedMonitorReports[0];
  const latestReport = evidence?.latest_report ?? sortedMonitorReports.at(-1);
  const latestTimelineItem = timeline.at(-1);
  const requestedTab = searchParams.get("tab");
  const activeTab: DetailTab = isDetailTab(requestedTab) ? requestedTab : "timeline";
  const canAcknowledge = incident?.status === "open";
  const canResolve = incident?.status !== "resolved";
  const canCover = incident?.status === "open" || incident?.status === "acknowledged";
  const canReopen = incident?.status === "resolved" || incident?.status === "covered";
  const actionPending =
    acknowledgeIncident.isPending ||
    resolveIncident.isPending ||
    coverIncident.isPending ||
    reopenIncident.isPending;

  const handleTabChange = (tab: string) => {
    if (!isDetailTab(tab)) return;
    setSearchParams(
      (params) => {
        params.set("tab", tab);
        return params;
      },
      { replace: true },
    );
  };

  const handleNextAction = (action: ApiIncidentNextActionResponse) => {
    const id = action.target_id || incident?.id || "";
    switch (action.action_type) {
      case "acknowledge_incident":
        acknowledgeIncident.mutate({ id });
        break;
      case "cover_incident":
        setCoverDialogOpen(true);
        break;
      case "resolve_incident":
        resolveIncident.mutate({ id });
        break;
      case "reopen_incident":
        reopenIncident.mutate({ id });
        break;
    }
  };

  if (incidentResponse.isLoading) {
    return <div className="py-3 text-sm text-neutral-600">Loading incident...</div>;
  }

  if (incidentResponse.error) {
    return <div className="py-3 text-sm">Unable to load incident.</div>;
  }

  if (!incident) {
    return <div className="py-3 text-sm text-neutral-600">Incident not found.</div>;
  }

  return (
    <div className="space-y-7">
      <div className="space-y-1">
        <PageBreadcrumbs
          items={[
            { label: "Incidents", to: "/incidents" },
            { label: incident.title ?? "Incident" },
          ]}
        />
      </div>

      <section className="space-y-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="space-y-2">
            <h1 className="text-base font-medium">{incident.title ?? "Untitled incident"}</h1>
            <p className="max-w-3xl text-sm text-neutral-600">
              {incident.latest_event ?? "No latest event recorded."}
            </p>
          </div>
          {nextActions.length === 0 && (canResolve || canReopen) && (
            <div className="flex flex-wrap gap-2">
              {canAcknowledge && (
                <Button
                  variant="outline"
                  disabled={actionPending}
                  onClick={() => acknowledgeIncident.mutate({ id: incident.id ?? "" })}
                >
                  <CheckIcon />
                  Acknowledge
                </Button>
              )}
              {canCover && (
                <Button
                  variant="outline"
                  disabled={actionPending}
                  onClick={() => setCoverDialogOpen(true)}
                >
                  <ShieldCheckIcon />
                  Cover
                </Button>
              )}
              {canResolve && (
                <Button
                  disabled={actionPending}
                  onClick={() => resolveIncident.mutate({ id: incident.id ?? "" })}
                >
                  <CircleCheckIcon />
                  Resolve
                </Button>
              )}
              {canReopen && (
                <Button
                  variant="outline"
                  disabled={actionPending}
                  onClick={() => reopenIncident.mutate({ id: incident.id ?? "" })}
                >
                  <RotateCcwIcon />
                  Reopen
                </Button>
              )}
            </div>
          )}
        </div>
        {(acknowledgeIncident.error ||
          resolveIncident.error ||
          coverIncident.error ||
          reopenIncident.error) && (
          <div className="text-sm text-rose-700">Unable to update incident.</div>
        )}

        <IncidentNextActionPanel
          actions={nextActions}
          actionPending={actionPending}
          incident={incident}
          onAction={handleNextAction}
        />

        <div className="grid gap-3 lg:grid-cols-3">
          <DetailGroup title="Incident">
            <DetailItem label="status" value={<StatusBadge value={toStatus(incident.status)} />} />
            <DetailItem
              label="severity"
              value={<SeverityBadge value={toSeverity(incident.severity)} />}
            />
            <DetailItem
              label="notification"
              value={
                <NotificationBadge
                  value={toNotificationStatus(incident.notification_status)}
                  fallback="no notification status"
                />
              }
            />
            <DetailItem label="duration" value={durationLabel(incident)} />
            <DetailItem
              label="resolution"
              value={incident.resolution_kind || (incident.status === "covered" ? "covered" : "—")}
            />
          </DetailGroup>

          <DetailGroup title="Affected">
            <DetailItem label="server" value={incident.agent_name ?? "Unknown server"} />
            <DetailItem label="monitor" value={incident.monitor_name ?? "Unknown monitor"} />
            <DetailItem label="monitor type" value={incident.monitor_type ?? "unknown"} />
            <DetailItem
              label="components"
              value={<ComponentImpactList components={impactedComponents} />}
            />
            <div className="flex flex-wrap gap-4 text-sm">
              {incident.agent_id && (
                <Link
                  to={`/servers/${incident.agent_id}?tab=monitors&incident=${encodeURIComponent(incident.id ?? "")}`}
                  className="font-medium hover:text-neutral-600"
                >
                  View server
                </Link>
              )}
              {incident.monitor_id && (
                <Link
                  to={`/monitors/${incident.monitor_id}?incident=${encodeURIComponent(incident.id ?? "")}`}
                  className="font-medium hover:text-neutral-600"
                >
                  View monitor
                </Link>
              )}
            </div>
          </DetailGroup>

          <DetailGroup title="Timing">
            <DetailItem label="opened" value={formatDate(incident.opened_at, DATE_TIME_FORMAT)} />
            <DetailItem
              label="latest event"
              value={formatDate(incident.last_event_at, DATE_TIME_FORMAT)}
            />
            <DetailItem
              label="resolved"
              value={formatDate(incident.resolved_at, DATE_TIME_FORMAT)}
            />
            <DetailItem
              label="covered until"
              value={formatDate(incident.covered_until, DATE_TIME_FORMAT)}
            />
            {incident.coverage_note && (
              <DetailItem label="coverage note" value={incident.coverage_note} />
            )}
          </DetailGroup>
        </div>
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Cause / Evidence</h2>
        <div className="grid gap-3 lg:grid-cols-3">
          <EvidenceReportGroup
            title="Trigger"
            report={triggeringReport}
            reason={reportReason(triggeringReport)}
            onInspect={() => setSelectedMonitorReport(triggeringReport)}
          />

          <EvidenceReportGroup
            title="Current Result"
            report={latestReport}
            reason={reportReason(latestReport)}
            onInspect={() => setSelectedMonitorReport(latestReport)}
          />

          <DetailGroup title="Latest Timeline Event">
            <DetailItem label="type" value={latestTimelineItem?.type ?? "—"} />
            <DetailItem
              label="time"
              value={formatDate(latestTimelineItem?.created_at, DATE_TIME_FORMAT)}
            />
            <DetailItem label="message" value={latestTimelineItem?.message ?? "—"} />
          </DetailGroup>
        </div>
        {relatedIncidents.length > 0 && (
          <div className="space-y-2 bg-neutral-50 px-3 py-3">
            <h3 className="text-sm font-medium">Related incidents</h3>
            <div className="divide-y divide-neutral-200">
              {relatedIncidents.slice(0, 5).map((related) => (
                <Link
                  key={related.id ?? related.opened_at ?? "related-incident"}
                  to={`/incidents/${related.id ?? ""}`}
                  className="grid gap-1 py-2 text-sm hover:text-neutral-600 sm:grid-cols-[1fr_auto]"
                >
                  <span className="min-w-0 truncate font-medium">
                    {related.title ?? "Untitled incident"}
                  </span>
                  <span className="text-neutral-500">
                    {formatDate(related.opened_at, DATE_TIME_FORMAT)}
                  </span>
                </Link>
              ))}
            </div>
          </div>
        )}
      </section>

      <section className="space-y-4">
        <h2 className="text-sm font-medium">Operational Data</h2>
        <Tabs value={activeTab} onValueChange={handleTabChange} className="space-y-3">
          <TabsList>
            <TabsTrigger value="timeline">
              Timeline <TabCount>{timeline.length}</TabCount>
            </TabsTrigger>
            <TabsTrigger value="notifications">
              Notifications <TabCount>{alertDeliveries.length}</TabCount>
            </TabsTrigger>
            <TabsTrigger value="monitor-reports">
              Monitor reports <TabCount>{monitorReports.length}</TabCount>
            </TabsTrigger>
          </TabsList>
          <TabsContent value="timeline">
            <DataTable
              columns={timelineColumns(reportsByID)}
              data={timeline}
              emptyMessage="No timeline events recorded."
              getRowId={(item, index) => item.id ?? `timeline-${index}`}
              onRowClick={(item) => {
                if (item.monitor_report_id) {
                  setSelectedMonitorReport(reportsByID.get(item.monitor_report_id));
                  return;
                }
                if (item.alert_delivery_id) {
                  handleTabChange("notifications");
                }
              }}
            />
          </TabsContent>
          <TabsContent value="notifications">
            <DataTable
              columns={notificationColumns}
              data={alertDeliveries}
              emptyMessage="No notification deliveries recorded."
              getRowId={(delivery, index) => delivery.id ?? `notification-${index}`}
            />
          </TabsContent>
          <TabsContent value="monitor-reports">
            <DataTable
              columns={monitorReportColumns}
              data={monitorReports}
              emptyMessage="No monitor reports linked."
              getRowId={(report, index) => report.id ?? `monitor-report-${index}`}
              onRowClick={setSelectedMonitorReport}
            />
          </TabsContent>
        </Tabs>
      </section>
      <ReportInspectionDrawer
        kind="monitor"
        report={selectedMonitorReport}
        onOpenChange={(open) => {
          if (!open) setSelectedMonitorReport(undefined);
        }}
      />
      <CoverIncidentDialog
        open={coverDialogOpen}
        pending={coverIncident.isPending}
        onOpenChange={setCoverDialogOpen}
        onSubmit={(payload) =>
          coverIncident.mutate(
            { id: incident.id ?? "", data: payload },
            { onSuccess: () => setCoverDialogOpen(false) },
          )
        }
      />
    </div>
  );
};
