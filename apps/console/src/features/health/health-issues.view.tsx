import { Link } from "react-router-dom";
import {
  Button,
  Dot,
  EmptyState,
  GridRow,
  GridTable,
  PageHeader,
  Skeleton,
  StatusBadge,
} from "@/design-system";
import { healthDot } from "@/features/incidents/incident-format";
import { useNow } from "@/features/overview/overview-time";
import { type ApiHealthIssue, useGetHealthIssues } from "@/orion-sdk";

const COLUMNS = "28px minmax(0,1.3fr) 110px 90px 110px minmax(0,1.5fr) 60px 120px";

const label = (value?: string) => (value ? value.charAt(0).toUpperCase() + value.slice(1) : "-");

const since = (iso: string | undefined, now: number) => {
  if (!iso) return "-";
  const minutes = Math.max(0, Math.floor((now - new Date(iso).getTime()) / 60000));
  if (minutes < 60) return `${Math.max(minutes, 1)}m`;
  const hours = Math.floor(minutes / 60);
  return hours < 24 ? `${hours}h` : `${Math.floor(hours / 24)}d`;
};

const Action = ({ issue }: { issue: ApiHealthIssue }) => {
  if (issue.incident_id) {
    return (
      <Link className="font-mono text-[12.5px] underline" to={`/incidents/${issue.incident_id}`}>
        {issue.incident_display_id ?? "Incident"}
      </Link>
    );
  }
  const isServer = issue.kind === "server";
  const id = issue.target_id ?? (isServer ? issue.agent_id : issue.monitor_id) ?? "";
  return (
    <Link to={isServer ? `/servers/${id}` : `/monitors/${id}`}>
      <Button size="sm" variant="secondary">
        {isServer ? "Open server" : "Open monitor"}
      </Button>
    </Link>
  );
};

export const HealthIssuesPage = () => {
  const issues = useGetHealthIssues();
  const now = useNow(60_000);
  const list = issues.data?.issues ?? [];
  return (
    <div className="mx-auto w-full max-w-[1000px] pb-8">
      <PageHeader
        title="Health issues"
        description="Monitors and servers that are down, degraded or stale, worst first."
      />
      <div className="mt-4">
        {issues.isLoading ? (
          <Skeleton className="h-40 w-full rounded-xl" />
        ) : issues.isError ? (
          <EmptyState
            tone="error"
            title="Unable to load health issues"
            action={
              <Button size="sm" variant="secondary" onClick={() => void issues.refetch()}>
                Retry
              </Button>
            }
          />
        ) : list.length === 0 ? (
          <EmptyState title="No health issues" description="Everything Orion watches is healthy." />
        ) : (
          <div className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
            <GridTable dividers>
              <GridRow columns={COLUMNS} header>
                <span />
                <span>Name</span>
                <span>Server</span>
                <span>Kind</span>
                <span>Status</span>
                <span>Detail</span>
                <span>Since</span>
                <span />
              </GridRow>
              {list.map((issue, index) => (
                <GridRow key={`${issue.kind}-${issue.target_id ?? index}`} columns={COLUMNS}>
                  <Dot color={healthDot(issue.health)} />
                  <b className="font-medium">
                    {issue.target_name ?? issue.monitor_name ?? issue.agent_name}
                  </b>
                  <span className="text-faint">{issue.agent_name ?? "Core"}</span>
                  <span className="text-faint">{label(issue.kind)}</span>
                  <span>
                    <StatusBadge status={issue.health}>
                      {issue.issue_type === "stale_data" ? "Stale" : label(issue.health)}
                    </StatusBadge>
                  </span>
                  <span className="text-faint">{issue.detail ?? "-"}</span>
                  <span className="text-faint">{since(issue.since, now)}</span>
                  <span className="flex justify-end !overflow-visible">
                    <Action issue={issue} />
                  </span>
                </GridRow>
              ))}
            </GridTable>
          </div>
        )}
      </div>
    </div>
  );
};
