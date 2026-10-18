import {
  DataTable,
  LifecycleTrack,
  TabCount,
  Tabs,
  TabsList,
  TabsPanel,
  TabsTab,
} from "@/design-system";
import { ReportInspectionDrawer } from "@/features/report-inspection/components/report-inspection-drawer";
import { useNow } from "@/features/overview/overview-time";
import {
  type ApiMonitorReportResponse,
  type ApiStatusPageIncidentResponse,
  getGetIncidentQueryKey,
  getGetIncidentTimelineQueryKey,
  getGetIncidentsQueryKey,
  getListStatusPagesQueryKey,
  useAcknowledgeIncident,
  useAddIncidentNote,
  useCoverIncident,
  useCreateStatusPageIncidentDraft,
  useGetIncident,
  useListStatusPages,
  usePreviewStatusPageIncidentDraft,
  useReopenIncident,
  useResolveIncident,
  useRetryAlertDelivery,
  useSetIncidentResponder,
} from "@/orion-sdk";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useParams, useSearchParams } from "react-router-dom";

import {
  ActionNoteDialog,
  CoverIncidentDialog,
  PublicIncidentDraftDialog,
} from "./components/incident-detail-dialogs";
import {
  type DetailTab,
  type IncidentWithAllowedActions,
  type LifecycleAction,
  actionAllowed,
  isDetailTab,
  monitorReportColumns,
  notificationColumns,
  reportSortTime,
} from "./components/incident-detail-utils";
import { ExpectedActual } from "./detail/expected-actual";
import { IncidentDetailHeader } from "./detail/incident-detail-header";
import { incidentMoments, lifecycleSteps } from "./detail/incident-lifecycle";
import { IncidentSidePanel } from "./detail/incident-side-panel";
import { IncidentTimeline, NoteComposer } from "./detail/incident-timeline";

export const IncidentDetailPage = () => {
  const { incidentId = "" } = useParams();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const refreshIncident = () => {
    void queryClient.invalidateQueries({ queryKey: getGetIncidentQueryKey(incidentId) });
    void queryClient.invalidateQueries({ queryKey: getGetIncidentTimelineQueryKey(incidentId) });
    void queryClient.invalidateQueries({ queryKey: getGetIncidentsQueryKey() });
  };
  const incidentResponse = useGetIncident(incidentId);
  const acknowledgeIncident = useAcknowledgeIncident({ mutation: { onSuccess: refreshIncident } });
  const resolveIncident = useResolveIncident({ mutation: { onSuccess: refreshIncident } });
  const coverIncident = useCoverIncident({ mutation: { onSuccess: refreshIncident } });
  const reopenIncident = useReopenIncident({ mutation: { onSuccess: refreshIncident } });
  const now = useNow(1000);
  const addNote = useAddIncidentNote({ mutation: { onSuccess: refreshIncident } });
  const setResponder = useSetIncidentResponder({ mutation: { onSuccess: refreshIncident } });
  const retryDelivery = useRetryAlertDelivery({ mutation: { onSuccess: refreshIncident } });
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
        void queryClient.invalidateQueries({ queryKey: getListStatusPagesQueryKey() });
      },
    },
  });
  const evidence = incidentResponse.data?.evidence;
  const timeline = incidentResponse.data?.timeline ?? [];
  const summary = incidentResponse.data?.summary;
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
  const latestReport = evidence?.latest_report ?? sortedMonitorReports.at(-1);
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
    return <div className="py-3 text-sm text-quiet">Loading incident...</div>;
  }

  if (incidentResponse.error) {
    return <div className="py-3 text-sm">Unable to load incident.</div>;
  }

  if (!incident) {
    return <div className="py-3 text-sm text-quiet">Incident not found.</div>;
  }

  const moments = incidentMoments(incident, alertDeliveries, timeline, summary);
  const monitorName = incident.monitor_name ?? "the monitor";

  return (
    <div className="mx-auto w-full max-w-[1040px]">
      <IncidentDetailHeader
        incident={incident}
        now={now}
        actionPending={actionPending}
        canAcknowledge={canAcknowledge}
        canCover={canCover}
        canReopen={canReopen}
        canResolve={canResolve}
        onAcknowledge={() => setActionDialog("acknowledge")}
        onCover={() => setCoverDialogOpen(true)}
        onReopen={() => setActionDialog("reopen")}
        onResolve={() => setActionDialog("resolve")}
      />
      {(acknowledgeIncident.error ||
        resolveIncident.error ||
        coverIncident.error ||
        reopenIncident.error) && (
        <div className="text-sm text-rose-700">Unable to update incident.</div>
      )}

      <div className="mt-3 grid gap-5 pb-6 lg:grid-cols-[minmax(0,1fr)_300px]">
        <div className="flex min-w-0 flex-col gap-4">
          <p className="text-sm leading-[1.6] text-mid">
            {incident.latest_event ?? `No events recorded for ${monitorName} yet.`}
          </p>
          <div className="rounded-xl border border-line-soft bg-field px-4 py-3.5">
            <LifecycleTrack steps={lifecycleSteps(incident, moments, now)} />
          </div>
          <ExpectedActual
            expected={summary?.expected}
            actual={summary?.actual}
            report={latestReport}
            now={now}
          />
          <Tabs value={activeTab} onValueChange={handleTabChange} className="gap-3.5">
            <TabsList className="w-full gap-2 self-stretch">
              <TabsTab value="timeline" className="px-2.5">
                Timeline
              </TabsTab>
              <TabsTab value="monitor-reports" className="px-2.5">
                Checks <TabCount className="ml-1">{monitorReports.length}</TabCount>
              </TabsTab>
              <TabsTab value="notifications" className="px-2.5">
                Notifications <TabCount className="ml-1">{alertDeliveries.length}</TabCount>
              </TabsTab>
            </TabsList>
            <TabsPanel value="timeline" className="flex flex-col gap-3.5">
              <NoteComposer
                pending={addNote.isPending}
                failed={addNote.isError}
                onPost={(text, done) =>
                  addNote.mutate({ id: incidentId, data: { text } }, { onSuccess: done })
                }
              />
              <IncidentTimeline
                items={timeline}
                retrying={retryDelivery.isPending}
                onRetry={(id) => retryDelivery.mutate({ id })}
                onOpen={(item) => {
                  if (item.monitor_report_id) {
                    setSelectedMonitorReport(reportsByID.get(item.monitor_report_id));
                  } else if (item.alert_delivery_id) {
                    handleTabChange("notifications");
                  }
                }}
              />
            </TabsPanel>
            <TabsPanel value="monitor-reports">
              <DataTable
                columns={monitorReportColumns}
                data={monitorReports}
                emptyMessage="No monitor reports linked."
                getRowId={(report, index) => report.id ?? `monitor-report-${index}`}
                onRowClick={setSelectedMonitorReport}
              />
            </TabsPanel>
            <TabsPanel value="notifications">
              <DataTable
                columns={notificationColumns}
                data={alertDeliveries}
                emptyMessage="No notification deliveries recorded."
                getRowId={(delivery, index) => delivery.id ?? `notification-${index}`}
              />
            </TabsPanel>
          </Tabs>
        </div>
        <IncidentSidePanel
          incident={incident}
          summary={summary}
          now={now}
          notifiedAt={moments.notifiedAt}
          acknowledgedAt={moments.acknowledgedAt}
          related={relatedIncidents}
          onDraftUpdate={() => handlePublicDraftDialogOpenChange(true)}
          onSetResponder={(name) => setResponder.mutate({ id: incidentId, data: { name } })}
        />
      </div>
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
