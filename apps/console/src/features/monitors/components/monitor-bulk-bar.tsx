import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Button, ConfirmDialog, FloatingBar, useToast } from "@/design-system";
import {
  type ApiBulkMonitorRequest,
  getGetMonitorSummaryQueryKey,
  getGetMonitorsQueryKey,
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
  onClear,
}: {
  ids: readonly string[];
  onClear: () => void;
}) => {
  const queryClient = useQueryClient();
  const toast = useToast();
  const bulk = useBulkMonitorAction();
  const [confirmDelete, setConfirmDelete] = useState(false);

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
    if (failedIds.length === 0) {
      toast.add({ title: `${done} monitors ${VERB[action] ?? action}` });
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
        variant="destructive"
        disabled={bulk.isPending}
        onClick={() => setConfirmDelete(true)}
      >
        Delete
      </Button>
      <Button size="sm" variant="ghost" onClick={onClear}>
        Clear
      </Button>
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
