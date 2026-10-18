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

  const run = async (action: BulkAction) => {
    let failed = ids.length;
    try {
      const result = await bulk.mutateAsync({
        data: {
          ids: [...ids],
          action,
          note: action === "cover" ? "Covered in bulk for 1 hour" : `Bulk ${action}`,
        },
      });
      failed = result.failed ?? 0;
    } catch {
      failed = ids.length;
    }
    void queryClient.invalidateQueries({ queryKey: getGetIncidentsQueryKey() });
    void queryClient.invalidateQueries({ queryKey: getGetOverviewQueryKey() });
    const done = ids.length - failed;
    toast.add({
      title: failed === 0 ? `${done} incidents updated` : `${done} updated, ${failed} failed`,
      description: failed === 0 ? undefined : "Open the failed incidents to see why.",
    });
    if (failed === 0) onClear();
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
