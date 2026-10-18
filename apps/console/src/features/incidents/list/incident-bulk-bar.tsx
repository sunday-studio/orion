import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Button, FloatingBar, useToast } from "@/design-system";
import {
  getGetIncidentsQueryKey,
  getGetOverviewQueryKey,
  useAcknowledgeIncident,
  useCoverIncident,
  useResolveIncident,
} from "@/orion-sdk";

type BulkAction = "acknowledge" | "cover" | "resolve";

const COVER_FOR_MS = 60 * 60 * 1000;

/**
 * Bulk actions run the single-incident endpoints one by one until Core gets a bulk route.
 * Failures are counted and reported so a partial result is never shown as success.
 */
export const IncidentBulkBar = ({
  ids,
  onClear,
}: {
  ids: readonly string[];
  onClear: () => void;
}) => {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [busy, setBusy] = useState(false);
  const acknowledge = useAcknowledgeIncident();
  const cover = useCoverIncident();
  const resolve = useResolveIncident();

  const run = async (action: BulkAction) => {
    setBusy(true);
    let failed = 0;
    for (const id of ids) {
      try {
        if (action === "acknowledge") await acknowledge.mutateAsync({ id });
        else if (action === "resolve")
          await resolve.mutateAsync({ id, data: { note: "Resolved in bulk" } });
        else
          await cover.mutateAsync({
            id,
            data: {
              covered_until: new Date(Date.now() + COVER_FOR_MS).toISOString(),
              note: "Covered in bulk for 1 hour",
            },
          });
      } catch {
        failed += 1;
      }
    }
    setBusy(false);
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
