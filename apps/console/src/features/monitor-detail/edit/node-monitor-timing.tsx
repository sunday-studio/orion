import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Button, Callout, FieldPanel, Select, Skeleton, useToast } from "@/design-system";
import { FAIL_AFTER, INTERVALS, everyLabel, withCurrent } from "@/features/monitors/node-timing";
import {
  getListNodeMonitorDefinitionsQueryKey,
  useListNodeMonitorDefinitions,
  useUpdateNodeMonitorDefinition,
} from "@/orion-sdk";

/** Check frequency of a Node monitor: editable when Orion manages it, otherwise managed in the Node's config file. */
export const NodeMonitorTiming = ({ nodeId, monitorId }: { nodeId: string; monitorId: string }) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const query = useListNodeMonitorDefinitions(nodeId);
  const definition = (query.data?.definitions ?? []).find((item) => item.monitor_id === monitorId);
  const [interval, setInterval] = useState("60");
  const [failAfter, setFailAfter] = useState("2");
  useEffect(() => {
    if (!definition) return;
    setInterval(String(definition.interval_seconds ?? 60));
    setFailAfter(String(definition.fail_after ?? 2));
  }, [definition]);
  const save = useUpdateNodeMonitorDefinition({
    mutation: {
      onSuccess: () => {
        void queryClient.invalidateQueries({
          queryKey: getListNodeMonitorDefinitionsQueryKey(nodeId),
        });
        toast.add({ title: "Saved. The Node applies it at its next sync, about a minute." });
      },
      onError: (failure) => toast.add({ title: (failure as Error).message || "Unable to save" }),
    },
  });

  if (query.isPending) return <Skeleton className="h-32 w-full rounded-xl" />;
  if (!definition) {
    return (
      <Callout tone="warn">
        This monitor is managed in a file on the server, so its timing is changed there. Run{" "}
        <code>orion-node config edit</code> on the server, the config is{" "}
        <code>/etc/orion/config.yaml</code> on Linux and{" "}
        <code>/usr/local/etc/orion/config.yaml</code> on macOS.
      </Callout>
    );
  }
  const dirty =
    interval !== String(definition.interval_seconds ?? 60) ||
    failAfter !== String(definition.fail_after ?? 2);
  return (
    <FieldPanel title="Timing" aside="Applied by the Node at its next sync">
      <div className="grid grid-cols-2 gap-4">
        <label className="grid gap-1.5 text-[12.5px] font-medium">
          Check
          <Select
            aria-label="Check frequency"
            className="w-full"
            options={withCurrent(INTERVALS, interval, everyLabel)}
            value={interval}
            onValueChange={setInterval}
          />
        </label>
        <label className="grid gap-1.5 text-[12.5px] font-medium">
          Down after
          <Select
            aria-label="Failed checks before down"
            className="w-full"
            options={FAIL_AFTER}
            value={failAfter}
            onValueChange={setFailAfter}
          />
        </label>
      </div>
      <div className="mt-3 flex justify-end">
        <Button
          disabled={!dirty || save.isPending}
          onClick={() =>
            save.mutate({
              id: nodeId,
              definitionId: definition.id ?? "",
              data: { interval_seconds: Number(interval), fail_after: Number(failAfter) },
            })
          }
        >
          Save
        </Button>
      </div>
    </FieldPanel>
  );
};
