import { Plus } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";
import { Button, EmptyState, FieldPanel, PageHeader, RaisedCard, Skeleton } from "@/design-system";
import { useGetHealthIssues, useGetMe, useGetOverview } from "@/orion-sdk";
import { AttentionBox, OngoingBox, RecentIncidentsBox, StatColumn } from "./overview-boxes";
import { OverviewFirstRun, isFirstRunDismissed } from "./overview-first-run";
import { NeedsYouNow } from "./needs-you-now";
import { formatHeaderDate, useNow } from "./overview-time";

const checkedAgo = (now: number, updatedAt: number) => {
  const seconds = Math.max(1, Math.round((now - (updatedAt || now)) / 1000));
  return `Checked ${seconds} ${seconds === 1 ? "second" : "seconds"} ago`;
};

export const OverviewPage = () => {
  const now = useNow(1000);
  const [dismissed, setDismissed] = useState(isFirstRunDismissed);
  const overview = useGetOverview();
  const me = useGetMe();
  const issues = useGetHealthIssues();

  const data = overview.data;
  const stats = data?.stats;
  const username = me.data?.username || "admin";

  if (data?.state === "empty" && !dismissed) {
    return <OverviewFirstRun onDismiss={() => setDismissed(true)} />;
  }

  const ongoing = data?.ongoing_incidents ?? [];
  const recent = data?.recent_incidents ?? [];
  const issueList = issues.data?.issues ?? [];
  const allClear =
    overview.isSuccess &&
    issues.isSuccess &&
    data?.state === "ok" &&
    !data.needs_you &&
    issueList.length === 0;

  return (
    <div>
      <PageHeader
        title={`How is it going, ${username}?`}
        description={formatHeaderDate(now)}
        actions={
          <Link to="/monitors?new=1">
            <Button size="sm">
              <Plus /> New monitor
            </Button>
          </Link>
        }
      />
      {overview.isError ? (
        <EmptyState
          className="mt-3"
          tone="error"
          title="Unable to load the overview"
          action={
            <Button size="sm" variant="secondary" onClick={() => void overview.refetch()}>
              Retry
            </Button>
          }
        />
      ) : (
        <div className="mt-3 grid grid-cols-[minmax(0,1fr)_300px] items-start gap-4 max-lg:grid-cols-1">
          <div className="flex flex-col gap-3">
            {overview.isLoading ? (
              <Skeleton className="h-40 w-full rounded-[14px]" />
            ) : data?.needs_you ? (
              <NeedsYouNow incident={data.needs_you} now={now} />
            ) : (
              allClear && (
                <FieldPanel
                  title="All systems normal"
                  aside={checkedAgo(now, overview.dataUpdatedAt)}
                >
                  <RaisedCard className="flex items-center gap-2.5 px-4 py-3.5 text-sm font-medium">
                    <span aria-hidden className="size-2 rounded-full bg-[#2a9f69]" />
                    {stats?.monitors_up ?? 0} monitors passing, {stats?.servers_reporting ?? 0}{" "}
                    servers reporting
                  </RaisedCard>
                </FieldPanel>
              )
            )}
            {issueList.length > 0 && (
              <AttentionBox
                issues={issueList}
                total={data?.health_issue_count ?? issues.data?.count ?? issueList.length}
              />
            )}
            {ongoing.length > 0 && (
              <OngoingBox
                incidents={ongoing}
                count={data?.ongoing_incident_count ?? ongoing.length}
                now={now}
                topId={data?.needs_you?.id}
              />
            )}
          </div>
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-2.5">
              <StatColumn
                monitorsUp={stats?.monitors_up ?? 0}
                monitorsTotal={stats?.monitors_total ?? 0}
                serversUp={stats?.servers_reporting ?? 0}
                serversTotal={stats?.servers_total ?? 0}
                openIncidents={data?.ongoing_incident_count ?? ongoing.length}
              />
            </div>
            {recent.length > 0 && <RecentIncidentsBox incidents={recent} now={now} />}
          </div>
        </div>
      )}
    </div>
  );
};
