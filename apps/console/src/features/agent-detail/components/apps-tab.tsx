import { useMemo, useState } from "react";
import {
  Button,
  EmptyState,
  FieldPanel,
  RaisedCard,
  Segmented,
  Skeleton,
  StatusBadge,
  useToast,
} from "@/design-system";
import { useGetAgentAppResources, useGetAgentDiscovery } from "@/orion-sdk";
import { formatDate } from "@/utils/date";
import { type Sort, isStale, minutesAgo, partition } from "../app-resources-model";
import { useDiscoveryRequest } from "./detected-actions";
import { DockerDisk, DockerVolumes, TopApps, TrackedPaths } from "./apps-lists";
import { TrackStorageDialog } from "./track-storage-dialog";

const SHOWN = 10;

const StateRow = ({
  label,
  badge,
  tone,
  words,
  action,
}: {
  label: string;
  badge: string;
  tone: "up" | "degraded" | "stale" | "unknown";
  words: string;
  action?: React.ReactNode;
}) => (
  <div className="grid min-h-[46px] grid-cols-[150px_200px_minmax(0,1fr)_auto] items-center gap-3 border-t border-line-soft px-3.5 py-1.5 text-[13px] first:border-t-0">
    <span className="text-[12.5px] text-quiet">{label}</span>
    <span>
      <StatusBadge status={tone}>{badge}</StatusBadge>
    </span>
    <span className="truncate">{words}</span>
    <span className="flex justify-end">{action}</span>
  </div>
);

/** Per-app usage from the agent's latest report: top apps, tracked paths and Docker disk. */
export const AppsTab = ({
  agentId,
  onOpenDetected,
}: {
  agentId: string;
  onOpenDetected: () => void;
}) => {
  const toast = useToast();
  const query = useGetAgentAppResources(agentId, { query: { refetchInterval: 30_000 } });
  const discovery = useGetAgentDiscovery(agentId).data?.discovery;
  const requests = useDiscoveryRequest(agentId);
  const [sort, setSort] = useState<Sort>("cpu");
  const [tracking, setTracking] = useState(false);
  const data = query.data?.app_resources;
  const canAct = Boolean(discovery?.capabilities?.includes("requests"));
  const groups = useMemo(() => partition(data?.series ?? [], sort), [data, sort]);

  if (query.isPending) return <Skeleton className="h-48 w-full rounded-xl" />;
  if (query.isError)
    return (
      <EmptyState
        tone="error"
        title="Unable to load app usage"
        action={
          <Button variant="secondary" onClick={() => void query.refetch()}>
            Retry
          </Button>
        }
      />
    );
  if (!data) {
    const expected = Boolean(discovery?.capabilities?.includes("app_resources"));
    return (
      <FieldPanel title="App usage">
        <RaisedCard>
          {expected ? (
            <StateRow
              label="No data yet"
              badge="Waiting for the agent"
              tone="degraded"
              words="The first report is on its way"
            />
          ) : (
            <StateRow
              label="Not sending"
              badge="Not sending app usage"
              tone="unknown"
              words="Update the agent, or turn it on in its config (app_resources)"
              action={
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => {
                    void navigator.clipboard.writeText("orion-agent config edit");
                    toast.add({ title: "Command copied" });
                  }}
                >
                  Copy command
                </Button>
              }
            />
          )}
        </RaisedCard>
      </FieldPanel>
    );
  }

  const stale = isStale(data.reported_at, data.interval_seconds);
  const dockerBlocked = (discovery?.kinds ?? []).find(
    (kind) =>
      kind.kind === "container" && kind.state && !["ok", "off", "disabled"].includes(kind.state),
  );
  const notices = [
    stale && (
      <StateRow
        key="stale"
        label="Stale"
        badge={`Last report ${minutesAgo(data.reported_at)} min ago`}
        tone="stale"
        words={`The numbers are from ${formatDate(data.reported_at, "HH:mm")}`}
      />
    ),
    dockerBlocked && (
      <StateRow
        key="docker"
        label="Docker blocked"
        badge="Docker not readable"
        tone="degraded"
        words="Container names, sizes and volumes are not shown"
        action={
          <Button size="sm" variant="secondary" onClick={onOpenDetected}>
            See the fix
          </Button>
        }
      />
    ),
    ...(data.notes ?? []).map((note) => (
      <StateRow key={note} label="Source skipped" badge="Partial" tone="unknown" words={note} />
    )),
  ].filter(Boolean);

  return (
    <div className="flex flex-col gap-3.5">
      <div className="flex items-center justify-between gap-3">
        <Segmented
          aria-label="Sort apps by"
          role="group"
          options={[
            { value: "cpu", label: "CPU" },
            { value: "memory", label: "Memory" },
          ]}
          value={sort}
          onValueChange={(next) => setSort(next as Sort)}
        />
        <span className="text-[12.5px] text-faint">
          Updated {formatDate(data.reported_at, "HH:mm:ss", "unknown")}
        </span>
      </div>
      {notices.length > 0 && <RaisedCard>{notices}</RaisedCard>}
      {groups.top.length > 0 ? (
        <TopApps rows={groups.top.slice(0, SHOWN)} total={groups.top.length} />
      ) : (
        <EmptyState
          className="rounded-xl border-solid bg-card"
          title="No apps measured yet"
          description="Processes, services and containers appear after the agent's next report."
        />
      )}
      <TrackedPaths rows={groups.paths} canTrack={canAct} onTrack={() => setTracking(true)} />
      <DockerVolumes rows={groups.volumes} />
      <DockerDisk rows={groups.docker} />
      <TrackStorageDialog
        open={tracking}
        pending={requests.pending}
        onOpenChange={setTracking}
        onTrack={(path) => requests.send("track_storage", { paths: [path] })}
      />
    </div>
  );
};
