import { Link } from "react-router-dom";
import { EmptyState } from "@/components/shared/empty-state";
import { PageHeader } from "@/components/shared/page-header";
import { SeverityBadge, toSeverity } from "@/components/shared/status-badges";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import {
  useGetHealthIssues,
  useGetHealthSummary,
  useGetIncidents,
  useGetOrionEvents,
} from "@/orion-sdk";
import { OverviewOnboardingCard } from "./overview-onboarding-card";

const issueText = (issue: Record<string, unknown>) => {
  const text = issue.message ?? issue.title ?? issue.summary ?? issue.type;
  return typeof text === "string" && text ? text : "Health issue";
};

const Stat = ({ label, value, to }: { label: string; value: number | string; to: string }) => (
  <Link className="bg-neutral-100 p-3 hover:bg-neutral-200/60" to={to}>
    <div className="text-sm text-neutral-600">{label}</div>
    <div className="text-lg font-medium tabular-nums">{value}</div>
  </Link>
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

  return (
    <div className="space-y-7">
      <PageHeader title="Overview" description="Current health across servers and monitors." />
      <OverviewOnboardingCard />

      <section aria-label="Health summary" className="grid gap-1 sm:grid-cols-5">
        <Stat label="servers" value={summary.data?.agents?.total ?? 0} to="/servers" />
        <Stat label="monitors up" value={monitors?.up ?? 0} to="/monitors" />
        <Stat label="monitors down" value={monitors?.down ?? 0} to="/monitors" />
        <Stat
          label="degraded or stale"
          value={(monitors?.degraded ?? 0) + (monitors?.stale ?? 0)}
          to="/monitors"
        />
        <Stat
          label="open incidents"
          value={incidents.data?.count ?? openIncidents.length}
          to="/incidents"
        />
      </section>

      <div className="grid gap-6 lg:grid-cols-2">
        <section aria-label="Open incidents" className="space-y-2">
          <h2 className="text-sm font-medium">Open incidents</h2>
          {openIncidents.length === 0 ? (
            <EmptyState className="min-h-32" title="No open incidents" />
          ) : (
            <ul className="divide-y divide-neutral-200">
              {openIncidents.map((incident) => (
                <li
                  className="flex items-center justify-between gap-3 py-2 text-sm"
                  key={incident.id}
                >
                  <Link
                    className="min-w-0 truncate font-medium underline"
                    to={`/incidents/${incident.id}`}
                  >
                    {incident.title ?? incident.id}
                  </Link>
                  <SeverityBadge value={toSeverity(incident.severity)} />
                </li>
              ))}
            </ul>
          )}
        </section>

        <section aria-label="Health issues" className="space-y-2">
          <h2 className="text-sm font-medium">Health issues</h2>
          {issueList.length === 0 ? (
            <EmptyState className="min-h-32" title="No health issues" />
          ) : (
            <ul className="divide-y divide-neutral-200 text-sm">
              {issueList.slice(0, 5).map((issue, index) => (
                <li className="py-2" key={index}>
                  {issueText(issue)}
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      <section aria-label="Recent events" className="space-y-2">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-medium">Recent events</h2>
          <Link className="text-sm underline" to="/logs">
            All logs
          </Link>
        </div>
        {recentEvents.length === 0 ? (
          <EmptyState className="min-h-32" title="No recent events" />
        ) : (
          <ul className="divide-y divide-neutral-200 text-sm">
            {recentEvents.map((event) => (
              <li className="flex gap-3 py-2" key={event.id}>
                <span className="shrink-0 text-neutral-600 tabular-nums">
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
