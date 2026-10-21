import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  Button,
  ConfirmDialog,
  EmptyState,
  FieldPanel,
  RaisedCard,
  Skeleton,
  StatusBadge,
  useToast,
} from "@/design-system";
import {
  getGetAgentDiscoveryQueryKey,
  getListAgentMonitorDefinitionsQueryKey,
  useAcceptDiscoveredItems,
  useGetAgentAppResources,
  useGetAgentDiscovery,
  useListAgentDiscoveryRequests,
} from "@/orion-sdk";
import { formatDate } from "@/utils/date";
import { bytes } from "../app-resources-model";
import { canAccept, groupItems, kindTitle } from "../detected-model";
import { useDiscoveryRequest } from "./detected-actions";
import { DetectedAdded } from "./detected-added";
import { DetectedBlocked } from "./detected-blocked";
import { DetectedItem } from "./detected-item";
import { DetectedRequests } from "./detected-requests";
import { DetectedReviewDialog } from "./detected-review-dialog";
import { DetectedSettings } from "./detected-settings";

const OPEN = ["queued", "delivered"];

/** "Detected on this server": what the agent found, accept it, dismiss it or change what is scanned. */
export const DetectedTab = ({
  agentId,
  onOpenApps,
}: {
  agentId: string;
  onOpenApps: () => void;
}) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const query = useGetAgentDiscovery(agentId);
  const requests = useDiscoveryRequest(agentId);
  const resources = useGetAgentAppResources(agentId);
  const sizes = new Map(
    (resources.data?.app_resources?.series ?? [])
      .filter((series) => series.kind === "path")
      .map((series) => [series.name, series]),
  );
  const requestList = useListAgentDiscoveryRequests(agentId);
  const discovery = query.data?.discovery;
  const items = useMemo(() => discovery?.items ?? [], [discovery]);
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [reviewing, setReviewing] = useState(false);
  const [dismissingAll, setDismissingAll] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const canAct = Boolean(discovery?.capabilities?.includes("requests"));
  const accept = useAcceptDiscoveredItems({
    mutation: {
      onSuccess: (result) => {
        void queryClient.invalidateQueries({ queryKey: getGetAgentDiscoveryQueryKey(agentId) });
        void queryClient.invalidateQueries({
          queryKey: getListAgentMonitorDefinitionsQueryKey(agentId),
        });
        setReviewing(false);
        const skipped = result.skipped?.length ?? 0;
        toast.add({
          title: `Requested ${result.created?.length ?? 0} monitors${skipped > 0 ? `, ${skipped} skipped` : ""}`,
        });
      },
      onError: (failure) => toast.add({ title: (failure as Error).message || "Unable to accept" }),
    },
  });

  // High-confidence suggestions are ticked by default.
  useEffect(() => {
    setSelected(
      new Set(
        items
          .filter((item) => canAccept(item) && item.confidence === "high")
          .map((item) => item.id ?? ""),
      ),
    );
  }, [items]);

  // What is waiting for the agent, so rows and switches show Pending instead of a result.
  const open = (requestList.data?.requests ?? []).filter((r) => OPEN.includes(r.status ?? ""));
  const pendingRow = (id?: string) => {
    const tracking = open.find(
      (r) =>
        r.type === "track_storage" &&
        (r.payload?.paths as string[] | undefined)?.includes(id?.replace(/^storage:/, "") ?? ""),
    );
    if (tracking) return "Waiting for the agent";
    const hit = open.find(
      (r) =>
        (r.type === "dismiss" || r.type === "ignore" || r.type === "restore") &&
        (r.payload?.ids as string[] | undefined)?.includes(id ?? ""),
    );
    return hit
      ? hit.type === "ignore"
        ? "Never suggest pending"
        : hit.type === "restore"
          ? "Restore pending"
          : "Dismiss pending"
      : undefined;
  };
  const pendingSwitches = open.flatMap((r) =>
    r.type === "set_scanning"
      ? ["scanning"]
      : r.type === "set_kind"
        ? [String(r.payload?.kind ?? "")]
        : [],
  );

  const active = items.filter((item) => !item.dismissed);
  const dismissed = items.filter((item) => item.dismissed);
  const suggestions = active.filter((item) => item.status !== "monitored");
  const chosen = active.filter((item) => canAccept(item) && selected.has(item.id ?? ""));
  const skippedLogs = suggestions.filter(
    (item) => item.kind === "log" && selected.has(item.id ?? ""),
  );
  const toggle = (id: string) =>
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  const storageSize = (item: { kind?: string; name?: string; status?: string }) => {
    if (item.kind !== "storage" || item.status !== "monitored") return undefined;
    const series = sizes.get(item.name);
    return series?.storage_bytes === undefined ? undefined : bytes(series.storage_bytes);
  };
  const act = (id: string | undefined, type: "dismiss" | "restore" | "ignore") =>
    requests.send(type, { ids: [id] });

  if (query.isPending) return <Skeleton className="h-48 w-full rounded-xl" />;
  if (query.isError)
    return (
      <EmptyState
        tone="error"
        title="Unable to load what was detected"
        action={
          <Button variant="secondary" onClick={() => void query.refetch()}>
            Retry
          </Button>
        }
      />
    );
  if (!discovery)
    return (
      <EmptyState
        className="rounded-xl border-solid bg-card"
        title="No scan reported yet"
        description="The agent sends what it finds after its first scan. Older agents do not send one, update the agent or add monitors by hand."
        action={
          <Link to="/monitors/new/agent">
            <Button variant="secondary">Create monitor</Button>
          </Link>
        }
      />
    );

  const problems = (discovery.kinds ?? []).filter(
    (kind) => kind.state && !["ok", "off", "disabled", "empty", "none"].includes(kind.state),
  );
  const scanningOff = discovery.enabled === false;

  return (
    <div className="flex flex-col gap-3.5">
      <div className="flex flex-wrap items-center justify-between gap-2 text-[13px] text-quiet">
        <span className="flex items-center gap-2">
          <b className="font-medium text-ink">{discovery.things ?? 0} found</b> ·{" "}
          {discovery.suggested ?? 0} suggested
          {(discovery.new_items ?? 0) > 0 && ` · ${discovery.new_items} new`}
          {problems.length > 0 && <StatusBadge status="degraded">Limited scan</StatusBadge>}
        </span>
        <span className="flex items-center gap-2">
          <span className="mr-1">
            Last scan {formatDate(discovery.scanned_at, "MMM d, HH:mm", "unknown")}
          </span>
          <Button
            size="sm"
            variant="secondary"
            disabled={!canAct || requests.pending || scanningOff}
            onClick={() => requests.send("rescan")}
          >
            Rescan now
          </Button>
          <Button size="sm" variant="secondary" onClick={() => setShowSettings((value) => !value)}>
            {showSettings ? "Hide scan settings" : "Scan settings"}
          </Button>
        </span>
      </div>
      {problems.map((kind) => (
        <DetectedBlocked
          key={kind.kind}
          kind={kind}
          canAct={canAct}
          onRescan={() => requests.send("rescan")}
          onSkip={() => requests.send("set_kind", { kind: kind.kind, enabled: false })}
        />
      ))}
      {showSettings && (
        <DetectedSettings
          enabled={!scanningOff}
          kinds={discovery.kinds ?? []}
          canAct={canAct}
          pending={pendingSwitches}
          onScanning={(enabled) => requests.send("set_scanning", { enabled })}
          onKind={(kind, enabled) => requests.send("set_kind", { kind, enabled })}
        />
      )}
      {scanningOff ? (
        <EmptyState
          className="rounded-xl border-solid bg-card"
          title="Scanning is off"
          description="Host metrics keep working. Turn scanning on in scan settings, or create monitors by hand."
          action={
            <Link to="/monitors/new/agent">
              <Button variant="secondary">Create monitor</Button>
            </Link>
          }
        />
      ) : active.length === 0 ? (
        <EmptyState
          className="rounded-xl border-solid bg-card"
          title="Nothing else detected"
          description="Host metrics are on. Rescan to look again."
        />
      ) : (
        <>
          {groupItems(active).map(([kind, group]) => (
            <FieldPanel key={kind} title={kindTitle(kind)} aside={`${group.length}`}>
              <RaisedCard>
                {group.map((item) => (
                  <DetectedItem
                    key={item.id}
                    item={item}
                    selectable={canAccept(item)}
                    checked={selected.has(item.id ?? "")}
                    canAct={canAct}
                    pending={pendingRow(item.id)}
                    size={storageSize(item)}
                    waitingSize={
                      item.kind === "storage" && item.status === "monitored" && !storageSize(item)
                    }
                    onViewApps={onOpenApps}
                    onToggle={() => toggle(item.id ?? "")}
                    onAction={(type) => act(item.id, type)}
                  />
                ))}
              </RaisedCard>
            </FieldPanel>
          ))}
          <div className="flex flex-wrap items-center justify-between gap-3">
            <Button
              variant="secondary"
              disabled={!canAct || suggestions.length === 0}
              onClick={() => setDismissingAll(true)}
            >
              Dismiss all
            </Button>
            <Button disabled={chosen.length === 0} onClick={() => setReviewing(true)}>
              Accept selected ({chosen.length})
            </Button>
          </div>
        </>
      )}
      {dismissed.length > 0 && (
        <FieldPanel title="Dismissed" aside={`${dismissed.length}`}>
          <RaisedCard>
            {dismissed.map((item) => (
              <DetectedItem
                key={item.id}
                item={item}
                selectable={false}
                checked={false}
                canAct={canAct}
                pending={pendingRow(item.id)}
                onToggle={() => undefined}
                onAction={(type) => act(item.id, type)}
              />
            ))}
          </RaisedCard>
        </FieldPanel>
      )}
      <DetectedAdded agentId={agentId} />
      <DetectedRequests
        agentId={agentId}
        onCancel={requests.cancel}
        onRetry={(request) =>
          requests.send(request.type ?? "rescan", request.payload as Record<string, unknown>)
        }
      />
      <DetectedReviewDialog
        open={reviewing}
        items={chosen}
        skippedLogs={skippedLogs}
        pending={accept.isPending}
        onOpenChange={setReviewing}
        onConfirm={() =>
          accept.mutate({ agentId, data: { item_ids: chosen.map((item) => item.id ?? "") } })
        }
      />
      <ConfirmDialog
        open={dismissingAll}
        onOpenChange={setDismissingAll}
        title={`Dismiss all ${suggestions.length} suggestions?`}
        description="They move to Dismissed. You can restore each one."
        confirmLabel="Dismiss all"
        tone="default"
        onConfirm={() => requests.send("dismiss", { ids: suggestions.map((item) => item.id) })}
      />
    </div>
  );
};
