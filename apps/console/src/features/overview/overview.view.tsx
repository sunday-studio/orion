import { useState } from "react";
import { Link } from "react-router-dom";
import { getUsername } from "@/api/client";
import { Button, EmptyState, FieldPanel, PageHeader, RaisedCard, Skeleton } from "@/design-system";
import { Plus } from "lucide-react";
import {
  useGetAgentSummary,
  useGetHealthIssues,
  useGetHealthSummary,
  useGetIncidents,
} from "@/orion-sdk";
import { AttentionBox, OngoingBox, RecentIncidentsBox, StatColumn } from "./overview-boxes";
import { ACTIVE_INCIDENT_PARAMS } from "./overview-queries";
import { OverviewFirstRun, isFirstRunDismissed } from "./overview-first-run";
import { formatHeaderDate, useNow } from "./overview-time";
import { NeedsYouNow, mostSevere } from "./needs-you-now";

export const OverviewPage = () => {
  const now = useNow(1000);
  const [dismissed, setDismissed] = useState(isFirstRunDismissed);
  const summary = useGetHealthSummary();
  const agentSummary = useGetAgentSummary();
  const issues = useGetHealthIssues();
  const active = useGetIncidents(ACTIVE_INCIDENT_PARAMS);
  const resolved = useGetIncidents({ status: "resolved", limit: 3 });

  const monitors = summary.data?.monitors;
  const monitorTotal = monitors?.total ?? 0;
  const serverTotal = agentSummary.data?.total ?? summary.data?.agents?.total ?? 0;
  const isEmpty = summary.isSuccess && monitorTotal === 0 && serverTotal === 0;
  const failed = summary.isError || active.isError || issues.isError;
  const loading = summary.isLoading || active.isLoading || issues.isLoading;

  if (isEmpty && !dismissed) return <OverviewFirstRun onDismiss={() => setDismissed(true)} />;

  const activeIncidents = active.data?.incidents ?? [];
  const top = mostSevere(activeIncidents.filter((incident) => incident.status === "open"));
  const issueList = issues.data?.issues ?? [];
  const allClear = !loading && !failed && activeIncidents.length === 0 && issueList.length === 0;

  return (
    <div>
      <PageHeader
        title={`How is it going, ${getUsername()}?`}
        description={formatHeaderDate(now)}
        actions={
          <Link to="/monitors?new=1">
            <Button size="sm">
              <Plus /> New monitor
            </Button>
          </Link>
        }
      />
      {failed ? (
        <EmptyState
          className="mt-3"
          tone="error"
          title="Unable to load the overview"
          action={
            <Button
              size="sm"
              variant="secondary"
              onClick={() => {
                void summary.refetch();
                void active.refetch();
                void issues.refetch();
              }}
            >
              Retry
            </Button>
          }
        />
      ) : (
        <div className="mt-3 grid grid-cols-[minmax(0,1fr)_300px] items-start gap-4 max-lg:grid-cols-1">
          <div className="flex flex-col gap-3">
            {loading ? (
              <Skeleton className="h-40 w-full rounded-[14px]" />
            ) : top ? (
              <NeedsYouNow incident={top} now={now} />
            ) : (
              allClear && (
                <FieldPanel
                  title="All systems normal"
                  aside={`Checked ${Math.max(1, Math.round((now - (summary.dataUpdatedAt || now)) / 1000))} seconds ago`}
                >
                  <RaisedCard className="flex items-center gap-2.5 px-4 py-3.5 text-sm font-medium">
                    <span aria-hidden className="size-2 rounded-full bg-[#2a9f69]" />
                    {monitorTotal} monitors passing, {serverTotal} servers reporting
                  </RaisedCard>
                </FieldPanel>
              )
            )}
            {issueList.length > 0 && (
              <AttentionBox issues={issueList} total={issues.data?.count ?? issueList.length} />
            )}
            {activeIncidents.length > 0 && (
              <OngoingBox
                incidents={activeIncidents}
                count={active.data?.count ?? 0}
                now={now}
                topId={top?.id}
              />
            )}
          </div>
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-2.5">
              <StatColumn
                monitorsUp={monitors?.up ?? 0}
                monitorsTotal={monitorTotal}
                serversUp={agentSummary.data?.up ?? 0}
                serversTotal={serverTotal}
                openIncidents={active.data?.count ?? activeIncidents.length}
              />
            </div>
            {(resolved.data?.incidents ?? []).length > 0 && (
              <RecentIncidentsBox incidents={resolved.data?.incidents ?? []} now={now} />
            )}
          </div>
        </div>
      )}
    </div>
  );
};
