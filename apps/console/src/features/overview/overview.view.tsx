import { Link } from "react-router-dom";
import { PageHeader } from "@/components/shared/page-header";
import {
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  EmptyState,
  Field,
  Skeleton,
  StatusBadge,
} from "@/design-system";
import {
  useGetHealthIssues,
  useGetHealthSummary,
  useGetIncidents,
  useGetOrionEvents,
} from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import { OverviewOnboardingCard } from "./overview-onboarding-card";

const issueText = (issue: Record<string, unknown>) => {
  const text = issue.message ?? issue.title ?? issue.summary ?? issue.type;
  return typeof text === "string" && text ? text : "Health issue";
};

const Stat = ({
  label,
  value,
  to,
  loading,
}: {
  label: string;
  value: number | string;
  to: string;
  loading: boolean;
}) => (
  <Link
    className="block rounded-[10px] outline-none focus-visible:ring-[3px] focus-visible:ring-focus"
    to={to}
  >
    <Card className="p-3 hover:bg-sub">
      <div className="text-xs text-quiet">{label}</div>
      {loading ? (
        <Skeleton className="mt-2 h-5 w-10" />
      ) : (
        <div className="text-lg font-medium tabular-nums">{value}</div>
      )}
    </Card>
  </Link>
);

const retry = (refetch: () => unknown) => (
  <Button size="sm" variant="secondary" onClick={() => void refetch()}>
    Retry
  </Button>
);

export const OverviewPage = () => {
  const summary = useGetHealthSummary();
  const issues = useGetHealthIssues();
  const incidents = useGetIncidents({ status: "open", limit: 5 });
  const events = useGetOrionEvents({ limit: 8 });

  const monitors = summary.data?.monitors;
  const openIncidents = incidents.data?.incidents ?? [];
  const issueList = issues.data?.issues ?? [];
  const recentEvents = events.data?.events ?? [];
  const allClear =
    !issues.isLoading &&
    !incidents.isLoading &&
    !issues.isError &&
    !incidents.isError &&
    issueList.length === 0 &&
    openIncidents.length === 0;

  return (
    <div className="space-y-7">
      <PageHeader title="Overview" description="Current health across servers and monitors." />
      <OverviewOnboardingCard />

      <Field aria-label="Needs you now">
        <h2 className="text-[15px] font-medium">
          {allClear ? "All systems normal" : "Needs you now"}
        </h2>
        {incidents.isError || issues.isError ? (
          <EmptyState
            tone="error"
            title="Unable to load current health"
            action={retry(() => {
              void incidents.refetch();
              void issues.refetch();
            })}
          />
        ) : (
          <div className="grid gap-3 lg:grid-cols-2">
            <Card aria-label="Open incidents">
              <CardHeader>
                <CardTitle>Open incidents</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                {incidents.isLoading ? (
                  <Skeleton className="m-4 h-4 w-2/3" />
                ) : openIncidents.length === 0 ? (
                  <p className="p-4 text-[13px] text-quiet">No open incidents.</p>
                ) : (
                  <ul className="divide-y divide-hairline">
                    {openIncidents.map((incident) => (
                      <li
                        className="flex items-center justify-between gap-3 px-4 py-2 text-[13px]"
                        key={incident.id}
                      >
                        <Link
                          className="min-w-0 truncate font-medium hover:underline"
                          to={`/incidents/${incident.id}`}
                        >
                          {incident.title ?? incident.id}
                        </Link>
                        <StatusBadge status={incident.severity} />
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>

            <Card aria-label="Health issues">
              <CardHeader>
                <CardTitle>Health issues</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                {issues.isLoading ? (
                  <Skeleton className="m-4 h-4 w-2/3" />
                ) : issueList.length === 0 ? (
                  <p className="p-4 text-[13px] text-quiet">No health issues.</p>
                ) : (
                  <ul className="divide-y divide-hairline text-[13px]">
                    {issueList.slice(0, 5).map((issue, index) => (
                      <li className="px-4 py-2" key={index}>
                        {issueText(issue)}
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
          </div>
        )}
      </Field>

      {summary.isError && (
        <EmptyState
          tone="error"
          title="Unable to load health summary"
          action={retry(summary.refetch)}
        />
      )}
      <section aria-label="Health summary" className="grid gap-3 sm:grid-cols-5">
        <Stat
          loading={summary.isLoading}
          label="servers"
          value={summary.data?.agents?.total ?? 0}
          to="/servers"
        />
        <Stat
          loading={summary.isLoading}
          label="monitors up"
          value={monitors?.up ?? 0}
          to="/monitors"
        />
        <Stat
          loading={summary.isLoading}
          label="monitors down"
          value={monitors?.down ?? 0}
          to="/monitors"
        />
        <Stat
          loading={summary.isLoading}
          label="degraded or stale"
          value={(monitors?.degraded ?? 0) + (monitors?.stale ?? 0)}
          to="/monitors"
        />
        <Stat
          loading={incidents.isLoading}
          label="open incidents"
          value={incidents.data?.count ?? openIncidents.length}
          to="/incidents"
        />
      </section>

      <section aria-label="Recent events" className="space-y-2">
        <div className="flex items-center justify-between">
          <h2 className="text-[15px] font-medium">Recent events</h2>
          <Link className="text-[13px] text-brand hover:underline" to="/logs">
            All logs
          </Link>
        </div>
        {events.isError ? (
          <EmptyState
            tone="error"
            title="Unable to load recent events"
            action={retry(events.refetch)}
          />
        ) : events.isLoading ? (
          <Skeleton className="h-4 w-1/2" />
        ) : recentEvents.length === 0 ? (
          <EmptyState title="No recent events" />
        ) : (
          <ul className="divide-y divide-hairline text-[13px]">
            {recentEvents.map((event) => (
              <li className="flex gap-3 py-2" key={event.id}>
                <span className="shrink-0 text-faint tabular-nums">
                  {formatDate(event.created_at, DATE_TIME_FORMAT)}
                </span>
                <span className="min-w-0 truncate">{event.message ?? event.type}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
};
