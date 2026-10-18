import { useQueryClient } from "@tanstack/react-query";
import { Button, FloatingBar, useToast } from "@/design-system";
import {
  getGetIncidentsQueryKey,
  getGetOverviewQueryKey,
  useBulkIncidentAction,
} from "@/orion-sdk";

type BulkAction = "acknowledge" | "cover" | "resolve";

export const IncidentBulkBar = ({
  ids,
  onClear,
}: {
  ids: readonly string[];
  onClear: () => void;
}) => {
  const queryClient = useQueryClient();
  const toast = useToast();
  const bulk = useBulkIncidentAction();
  const busy = bulk.isPending;

  const run = async (action: BulkAction, target: readonly string[] = ids) => {
    let failedIds: string[] = [...target];
    const reasons = new Map<string, string>();
    try {
      const result = await bulk.mutateAsync({
        data: {
          ids: [...target],
          action,
          note: action === "cover" ? "Covered in bulk for 1 hour" : `Bulk ${action}`,
        },
      });
      failedIds = (result.results ?? []).filter((r) => !r.ok).map((r) => r.id ?? "");
      for (const r of result.results ?? []) if (!r.ok) reasons.set(r.id ?? "", r.error ?? "failed");
    } catch {
      failedIds = [...target];
    }
    void queryClient.invalidateQueries({ queryKey: getGetIncidentsQueryKey() });
    void queryClient.invalidateQueries({ queryKey: getGetOverviewQueryKey() });
    const done = target.length - failedIds.length;
    const verb = { acknowledge: "acknowledged", cover: "covered", resolve: "resolved" }[action];
    if (failedIds.length === 0) {
      toast.add({ title: `${done} incidents ${verb}` });
      onClear();
      return;
    }
    toast.add({
      title: `${done} of ${target.length} ${verb}`,
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
      <Button size="sm" variant="secondary" disabled={busy} onClick={() => void run("acknowledge")}>
        Acknowledge
      </Button>
      <Button size="sm" variant="secondary" disabled={busy} onClick={() => void run("cover")}>
        Cover
      </Button>
      <Button size="sm" disabled={busy} onClick={() => void run("resolve")}>
        Resolve
      </Button>
      <Button size="sm" variant="ghost" disabled={busy} onClick={onClear}>
        Clear
      </Button>
    </FloatingBar>
  );
};
