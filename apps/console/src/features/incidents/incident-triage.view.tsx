import { useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import {
  Button,
  Dot,
  EmptyState,
  GridRow,
  GridTable,
  ListPagination,
  PageHeader,
  Skeleton,
  StatusBadge,
  useClientPagination,
  useToast,
} from "@/design-system";
import {
  type ApiIncidentCandidate,
  getGetIncidentsCandidatesQueryKey,
  getGetIncidentsQueryKey,
  getGetOverviewQueryKey,
  useDismissIncidentCandidate,
  useGetIncidentsCandidates,
  useOpenIncidentCandidate,
} from "@/orion-sdk";
import { TIME_FORMAT, formatDate } from "@/utils/date";
import { healthDot } from "./incident-format";

const COLUMNS = "10px minmax(0,1.3fr) 90px 80px 100px 90px minmax(0,1.4fr) 56px 150px";

const label = (value?: string) => (value ? value.charAt(0).toUpperCase() + value.slice(1) : "-");

const typeLabel = (type?: string) => (!type ? "Agent" : type === "http" ? "HTTP" : label(type));

export const IncidentTriagePage = () => {
  const queryClient = useQueryClient();
  const toast = useToast();
  const navigate = useNavigate();
  const candidatesQuery = useGetIncidentsCandidates();
  const candidates = candidatesQuery.data?.candidates ?? [];
  const { pageItems, pagination } = useClientPagination(candidates);

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: getGetIncidentsCandidatesQueryKey() });
    void queryClient.invalidateQueries({ queryKey: getGetIncidentsQueryKey() });
    void queryClient.invalidateQueries({ queryKey: getGetOverviewQueryKey() });
  };
  const dismiss = useDismissIncidentCandidate({
    mutation: {
      onSuccess: () => {
        refresh();
        toast.add({
          title: "Candidate dismissed",
          description: "It returns if the monitor fails again.",
        });
      },
      onError: () => toast.add({ title: "Unable to dismiss candidate" }),
    },
  });
  const open = useOpenIncidentCandidate({
    mutation: {
      onSuccess: (result) => {
        refresh();
        if (result.incident?.id) navigate(`/incidents/${result.incident.id}`);
      },
      onError: () => toast.add({ title: "Unable to open the incident" }),
    },
  });

  return (
    <div className="mx-auto w-full max-w-[1000px] pb-8">
      <PageHeader
        title="Triage"
        description="Signals Orion has not opened an incident for. Severity comes from recent failures."
        actions={
          candidates.length > 0 ? (
            <span className="text-[13px] text-quiet">{candidates.length} in the inbox</span>
          ) : (
            <Link className="text-[13px] text-quiet underline" to="/incidents">
              All incidents
            </Link>
          )
        }
      />
      <div className="mt-4">
        {candidatesQuery.isLoading ? (
          <Skeleton className="h-40 w-full rounded-xl" />
        ) : candidatesQuery.isError ? (
          <EmptyState
            tone="error"
            title="Unable to load triage"
            action={
              <Button size="sm" variant="secondary" onClick={() => void candidatesQuery.refetch()}>
                Retry
              </Button>
            }
          />
        ) : candidates.length === 0 ? (
          <EmptyState title="Nothing to triage" description="No failing monitors need attention." />
        ) : (
          <div className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
            <GridTable>
              <GridRow columns={COLUMNS} header>
                <span />
                <span>Monitor</span>
                <span>Server</span>
                <span>Type</span>
                <span>Health</span>
                <span>Severity</span>
                <span>Detail</span>
                <span>Seen</span>
                <span />
              </GridRow>
              {pageItems.map((candidate: ApiIncidentCandidate) => {
                const id = candidate.monitor_id ?? "";
                const name = candidate.monitor_name ?? id;
                return (
                  <GridRow key={id} columns={COLUMNS}>
                    <Dot color={healthDot(candidate.health)} />
                    <b className="font-medium">
                      {candidate.agent_name && candidate.monitor_type === "agent"
                        ? `${candidate.agent_name} ${name}`
                        : name}
                    </b>
                    <span className="text-faint">{candidate.agent_name ?? "Core"}</span>
                    <span className="text-faint">{typeLabel(candidate.monitor_type)}</span>
                    <span>
                      <StatusBadge status={candidate.health}>{label(candidate.health)}</StatusBadge>
                    </span>
                    <span>
                      <StatusBadge status={candidate.severity}>
                        {label(candidate.severity)}
                      </StatusBadge>
                    </span>
                    <span className="text-faint">
                      {candidate.detail ??
                        (candidate.down_count ? `${candidate.down_count} of 5 reports down` : "-")}
                    </span>
                    <span className="font-mono text-xs text-faint">
                      {formatDate(candidate.seen_at, TIME_FORMAT)}
                    </span>
                    <span className="flex justify-end gap-1.5 !overflow-visible">
                      <Button
                        size="sm"
                        variant="secondary"
                        disabled={dismiss.isPending}
                        onClick={() => dismiss.mutate({ id })}
                      >
                        Dismiss
                      </Button>
                      <Button
                        size="sm"
                        disabled={open.isPending}
                        onClick={() => open.mutate({ id })}
                      >
                        Open incident
                      </Button>
                    </span>
                  </GridRow>
                );
              })}
            </GridTable>
            <ListPagination {...pagination} />
          </div>
        )}
      </div>
    </div>
  );
};
