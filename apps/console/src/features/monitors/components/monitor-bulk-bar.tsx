import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Button, ConfirmDialog, FloatingBar, useToast } from "@/design-system";
import { MonitorIntervalDialog } from "./monitor-interval-dialog";
import {
  type ApiBulkMonitorRequest,
  bulkUpdateNodeMonitorDefinitions,
  getGetMonitorSummaryQueryKey,
  getGetMonitorsQueryKey,
  getListNodeMonitorDefinitionsQueryKey,
  listNodeMonitorDefinitions,
  useBulkMonitorAction,
} from "@/orion-sdk";

type Action = ApiBulkMonitorRequest["action"];

const VERB: Record<string, string> = {
  pause: "paused",
  resume: "resumed",
  maintenance: "put in maintenance",
  delete: "deleted",
  clone: "cloned",
};

export const MonitorBulkBar = ({
  ids,
  nodeIds,
  onClear,
  onSucceeded,
}: {
  ids: readonly string[];
  /** Monitor id to the server it runs on, for monitors that run through a Node. */
  nodeIds: Record<string, string>;
  onClear: () => void;
  /** Ids that succeeded, so the page can unselect them after a partial failure. */
  onSucceeded: (ids: readonly string[]) => void;
}) => {
  const queryClient = useQueryClient();
  const toast = useToast();
  const bulk = useBulkMonitorAction();
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [timing, setTiming] = useState(false);
  const [applying, setApplying] = useState(false);
  const nodeMonitors = ids.filter((id) => nodeIds[id]);

  // Node monitors managed by Orion have a definition; change them one server at a time.
  const changeTiming = async (interval: number, failAfter?: number) => {
    setApplying(true);
    let changed = 0;
    let failed = 0;
    const byNode = new Map<string, string[]>();
    for (const id of nodeMonitors)
      byNode.set(nodeIds[id], [...(byNode.get(nodeIds[id]) ?? []), id]);
    for (const [nodeId, monitorIds] of byNode) {
      try {
        const list = await listNodeMonitorDefinitions(nodeId);
        const definitionIds = (list.definitions ?? [])
          .filter((definition) => monitorIds.includes(definition.monitor_id ?? ""))
          .map((definition) => definition.id ?? "");
        if (definitionIds.length === 0) continue;
        await bulkUpdateNodeMonitorDefinitions(nodeId, {
          ids: definitionIds,
          interval_seconds: interval,
          ...(failAfter ? { fail_after: failAfter } : {}),
        });
        changed += definitionIds.length;
        void queryClient.invalidateQueries({
          queryKey: getListNodeMonitorDefinitionsQueryKey(nodeId),
        });
      } catch {
        failed += monitorIds.length;
      }
    }
    setApplying(false);
    setTiming(false);
    const skipped = ids.length - changed - failed;
    toast.add({
      title: `Changed ${changed} ${changed === 1 ? "monitor" : "monitors"}`,
      description:
        [
          skipped > 0
            ? `${skipped} skipped: only monitors on a server that Orion manages can change here.`
            : "",
          failed > 0 ? `${failed} failed.` : "",
        ]
          .filter(Boolean)
          .join(" ") || "The Node applies it at its next sync, about a minute.",
    });
  };

  const run = async (action: Action, target: readonly string[] = ids) => {
    let failedIds = [...target];
    const reasons = new Map<string, string>();
    try {
      const result = await bulk.mutateAsync({
        data: {
          ids: [...target],
          action,
          ...(action === "maintenance"
            ? { maintenance: { duration_seconds: 3600, reason: "Bulk maintenance" } }
            : {}),
          ...(action === "delete" ? { keep_history: true } : {}),
        },
      });
      failedIds = (result.results ?? []).filter((r) => !r.ok).map((r) => r.id ?? "");
      for (const r of result.results ?? []) if (!r.ok) reasons.set(r.id ?? "", r.error ?? "failed");
    } catch {
      failedIds = [...target];
    }
    void queryClient.invalidateQueries({ queryKey: getGetMonitorsQueryKey() });
    void queryClient.invalidateQueries({ queryKey: getGetMonitorSummaryQueryKey() });
    const done = target.length - failedIds.length;
    onSucceeded(target.filter((id) => !failedIds.includes(id)));
    if (failedIds.length === 0) {
      toast.add({
        title: `${done} ${done === 1 ? "monitor" : "monitors"} ${VERB[action] ?? action}`,
        description: action === "maintenance" ? "For one hour." : undefined,
      });
      onClear();
      return;
    }
    toast.add({
      title: `${done} of ${target.length} ${VERB[action] ?? action}`,
      description: failedIds
        .map((id) => `${id}: ${reasons.get(id) ?? "request failed"}`)
        .join("\n"),
      timeout: 10000,
      actionProps: { children: "Retry failed", onClick: () => void run(action, failedIds) },
    });
  };

  return (
    <FloatingBar aria-label="Bulk actions">
      <span className="pr-1 font-medium">{ids.length} selected</span>
      <span aria-hidden className="mx-1 h-5 w-px bg-hairline" />
      <Button
        size="sm"
        variant="secondary"
        disabled={bulk.isPending}
        onClick={() => void run("pause")}
      >
        Pause
      </Button>
      <Button
        size="sm"
        variant="secondary"
        disabled={bulk.isPending}
        onClick={() => void run("maintenance")}
      >
        Maintenance
      </Button>
      <Button
        size="sm"
        variant="secondary"
        disabled={bulk.isPending}
        onClick={() => void run("clone")}
      >
        Clone
      </Button>
      <Button
        size="sm"
        variant="secondary"
        disabled={nodeMonitors.length === 0}
        title={nodeMonitors.length === 0 ? "Select monitors that run on a server" : undefined}
        onClick={() => setTiming(true)}
      >
        Change interval
      </Button>
      <Button
        size="sm"
        variant="destructive"
        disabled={bulk.isPending}
        onClick={() => setConfirmDelete(true)}
      >
        Delete
      </Button>
      <Button size="sm" variant="ghost" onClick={onClear}>
        Clear
      </Button>
      <MonitorIntervalDialog
        open={timing}
        count={nodeMonitors.length}
        pending={applying}
        onOpenChange={setTiming}
        onApply={(interval, failAfter) => void changeTiming(interval, failAfter)}
      />
      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        title={`Delete ${ids.length} ${ids.length === 1 ? "monitor" : "monitors"}?`}
        description="Their check history is kept for 90 days. Open incidents stay."
        confirmLabel="Delete"
        onConfirm={() => void run("delete")}
      />
    </FloatingBar>
  );
};
