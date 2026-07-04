import { DataTable } from "@/components/shared/data-table";
import { DetailCard } from "@/components/shared/detail-card";
import { PageBreadcrumbs } from "@/components/shared/page-breadcrumbs";
import { TabCount, Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ReportInspectionDrawer } from "@/features/report-inspection/components/report-inspection-drawer";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import {
  type ApiMonitorReportResponse,
  type ApiStatusPageIncidentResponse,
  getGetIncidentQueryKey,
  getGetIncidentTimelineQueryKey,
  useAcknowledgeIncident,
  useCoverIncident,
  useCreateStatusPageIncidentDraft,
  useGetIncident,
  useListStatusPages,
  usePreviewStatusPageIncidentDraft,
  useReopenIncident,
  useResolveIncident,
} from "@/orion-sdk";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";

import {
  ActionNoteDialog,
  CoverIncidentDialog,
  EvidenceReportGroup,
  PublicIncidentDraftDialog,
} from "./components/incident-detail-dialogs";
import { IncidentActionButtons } from "./components/incident-action-buttons";
import {
  type DetailTab,
  type IncidentWithAllowedActions,
  type LifecycleAction,
  actionAllowed,
  isDetailTab,
  monitorReportColumns,
  notificationColumns,
  reportReason,
  reportSortTime,
  timelineColumns,
} from "./components/incident-detail-utils";
import { IncidentEvidenceCards, IncidentOverviewCards } from "./components/incident-summary-cards";

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
  const [publicDraftDialogOpen, setPublicDraftDialogOpen] = useState(false);
  const [selectedStatusPageID, setSelectedStatusPageID] = useState("");
  const [createdPublicDraft, setCreatedPublicDraft] = useState<ApiStatusPageIncidentResponse>();
  const statusPagesResponse = useListStatusPages({
    query: { enabled: publicDraftDialogOpen },
  });
  const incident = incidentResponse.data?.incident;
  const incidentID = incident?.id ?? "";
  const statusPages = statusPagesResponse.data?.pages ?? [];
  const firstStatusPageID = statusPagesResponse.data?.pages?.[0]?.id ?? "";
  const publicDraftPreview = usePreviewStatusPageIncidentDraft(
    selectedStatusPageID,
    { incident_id: incidentID },
    {
      query: {
        enabled: publicDraftDialogOpen && selectedStatusPageID !== "" && incidentID !== "",
      },
    },
  );
  const createPublicDraft = useCreateStatusPageIncidentDraft({
    mutation: {
      onSuccess: (data) => {
        setCreatedPublicDraft(data.incident);
        void queryClient.invalidateQueries({ queryKey: ["/v1/status-pages"] });
      },
    },
  });
  const impactedComponents = incident?.impacted_components ?? [];
  const evidence = incidentResponse.data?.evidence;
  const timeline = incidentResponse.data?.timeline ?? [];
  const alertDeliveries = incidentResponse.data?.alert_deliveries ?? [];
  const monitorReports = incidentResponse.data?.monitor_reports ?? [];
  const relatedIncidents = incidentResponse.data?.related_incidents ?? [];
  const [selectedMonitorReport, setSelectedMonitorReport] = useState<ApiMonitorReportResponse>();
  const [coverDialogOpen, setCoverDialogOpen] = useState(false);
  const [actionDialog, setActionDialog] = useState<LifecycleAction | null>(null);
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
  const incidentWithActions = incident as IncidentWithAllowedActions | undefined;
  const allowedActions = incidentWithActions?.allowed_actions;
  const canAcknowledge = allowedActions
    ? actionAllowed(allowedActions.acknowledge)
    : incident?.status === "open";
  const canResolve = allowedActions
    ? actionAllowed(allowedActions.resolve)
    : incident?.status !== "resolved";
  const canCover = allowedActions
    ? actionAllowed(allowedActions.cover)
    : incident?.status === "open" || incident?.status === "acknowledged";
  const canReopen = allowedActions
    ? actionAllowed(allowedActions.reopen)
    : incident?.status === "resolved" || incident?.status === "covered";
  const actionPending =
    acknowledgeIncident.isPending ||
    resolveIncident.isPending ||
    coverIncident.isPending ||
    reopenIncident.isPending ||
    createPublicDraft.isPending;

  useEffect(() => {
    if (!publicDraftDialogOpen || selectedStatusPageID || firstStatusPageID === "") return;
    setSelectedStatusPageID(firstStatusPageID);
  }, [firstStatusPageID, publicDraftDialogOpen, selectedStatusPageID]);

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

  const handleLifecycleAction = (payload: { note?: string }) => {
    const id = incident?.id ?? "";
    const data = payload.note ? payload : undefined;
    const onSuccess = () => setActionDialog(null);
    if (actionDialog === "acknowledge") {
      acknowledgeIncident.mutate({ id, data }, { onSuccess });
    }
    if (actionDialog === "resolve") {
      resolveIncident.mutate({ id, data }, { onSuccess });
    }
    if (actionDialog === "reopen") {
      reopenIncident.mutate({ id, data }, { onSuccess });
    }
  };

  const handlePublicDraftDialogOpenChange = (open: boolean) => {
    setPublicDraftDialogOpen(open);
    if (open) {
      setCreatedPublicDraft(undefined);
    }
  };

  const handleCreatePublicDraft = () => {
    const draft = publicDraftPreview.data?.draft;
    if (!incidentID || !selectedStatusPageID || !draft) return;
    createPublicDraft.mutate({
      id: selectedStatusPageID,
      data: {
        internal_incident_id: incidentID,
        affected_component_ids: draft.affected_component_ids ?? [],
      },
    });
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
          <IncidentActionButtons
            actionPending={actionPending}
            canAcknowledge={canAcknowledge}
            canCover={canCover}
            canReopen={canReopen}
            canResolve={canResolve}
            onAcknowledge={() => setActionDialog("acknowledge")}
            onCover={() => setCoverDialogOpen(true)}
            onOpenPublicDraft={() => handlePublicDraftDialogOpenChange(true)}
            onReopen={() => setActionDialog("reopen")}
            onResolve={() => setActionDialog("resolve")}
          />
        </div>
        {(acknowledgeIncident.error ||
          resolveIncident.error ||
          coverIncident.error ||
          reopenIncident.error) && (
          <div className="text-sm text-rose-700">Unable to update incident.</div>
        )}

        <IncidentOverviewCards incident={incident} impactedComponents={impactedComponents} />
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

          <IncidentEvidenceCards latestTimelineItem={latestTimelineItem} />
        </div>
        {relatedIncidents.length > 0 && (
          <DetailCard title="Related incidents" variant="violet">
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
          </DetailCard>
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
      <ActionNoteDialog
        action={actionDialog}
        pending={actionPending}
        onOpenChange={(open) => {
          if (!open) setActionDialog(null);
        }}
        onSubmit={handleLifecycleAction}
      />
      <PublicIncidentDraftDialog
        open={publicDraftDialogOpen}
        pages={statusPages}
        selectedStatusPageID={selectedStatusPageID}
        draft={publicDraftPreview.data?.draft}
        createdIncident={createdPublicDraft}
        loadingPages={statusPagesResponse.isLoading}
        loadingDraft={publicDraftPreview.isLoading}
        pending={createPublicDraft.isPending}
        hasError={Boolean(
          statusPagesResponse.error || publicDraftPreview.error || createPublicDraft.error,
        )}
        onOpenChange={handlePublicDraftDialogOpenChange}
        onStatusPageChange={(statusPageID) => {
          setSelectedStatusPageID(statusPageID);
          setCreatedPublicDraft(undefined);
        }}
        onCreate={handleCreatePublicDraft}
      />
    </div>
  );
};
