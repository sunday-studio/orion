import { useQueryClient } from "@tanstack/react-query";
import {
  Button,
  FieldPanel,
  GridRow,
  GridTable,
  RaisedCard,
  StatusBadge,
  useToast,
} from "@/design-system";
import {
  getListNodeMonitorDefinitionsQueryKey,
  useDeleteNodeMonitorDefinition,
  useListNodeMonitorDefinitions,
} from "@/orion-sdk";
import { formatDate } from "@/utils/date";

const COLUMNS = "minmax(0,1.2fr) minmax(0,1fr) 220px 80px";

const WHAT: Record<string, string> = {
  disk: "Above 90% full",
  port: "Listening",
  process: "Running",
  memory: "Memory",
  systemd: "Running",
  docker: "Running",
  pm2: "Online",
};

const status = (definition: {
  state?: string;
  delivered_at?: string;
  first_report_at?: string;
}) => {
  if (definition.state === "reporting") return { tone: "up" as const, text: "Reporting" };
  if (definition.state === "failed") return { tone: "down" as const, text: "Could not apply" };
  return definition.delivered_at
    ? { tone: "degraded" as const, text: "Waiting for first check" }
    : { tone: "degraded" as const, text: "Waiting for the Node" };
};

/** Monitors added from Detected: requested, applied by the node, then reporting. */
export const DetectedAdded = ({ nodeId }: { nodeId: string }) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const query = useListNodeMonitorDefinitions(nodeId, {
    query: {
      refetchInterval: (state) =>
        (state.state.data?.definitions ?? []).some((d) => d.state !== "reporting") ? 8000 : false,
    },
  });
  const remove = useDeleteNodeMonitorDefinition({
    mutation: {
      onSuccess: () =>
        void queryClient.invalidateQueries({
          queryKey: getListNodeMonitorDefinitionsQueryKey(nodeId),
        }),
      onError: () => toast.add({ title: "Unable to remove it" }),
    },
  });
  const definitions = query.data?.definitions ?? [];
  if (definitions.length === 0) return null;
  const count = (state: string) => definitions.filter((d) => d.state === state).length;
  return (
    <FieldPanel
      title="Added from Detected"
      aside={`${definitions.length} requested · ${count("reporting")} reporting · ${count("failed")} failed`}
    >
      <RaisedCard>
        <GridTable>
          <GridRow columns={COLUMNS} header>
            <span>Monitor</span>
            <span>Checks</span>
            <span>Status</span>
            <span />
          </GridRow>
          {definitions.map((definition) => {
            const current = status(definition);
            return (
              <GridRow key={definition.id} columns={COLUMNS}>
                <b className="font-medium">{definition.name}</b>
                <span className="text-mid">
                  {WHAT[definition.check_type ?? ""] ?? definition.check_type}
                </span>
                <span className="flex items-center gap-2">
                  <StatusBadge status={current.tone}>{current.text}</StatusBadge>
                  {definition.state === "failed" && definition.failure_reason && (
                    <span className="truncate text-xs text-faint" title={definition.failure_reason}>
                      {definition.failure_reason}
                    </span>
                  )}
                  {definition.state === "reporting" && (
                    <span className="text-xs text-faint">
                      {formatDate(definition.first_report_at, "HH:mm", "")}
                    </span>
                  )}
                </span>
                <span className="text-right">
                  {definition.state !== "reporting" && (
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() =>
                        remove.mutate({ id: nodeId, definitionId: definition.id ?? "" })
                      }
                    >
                      Cancel
                    </Button>
                  )}
                </span>
              </GridRow>
            );
          })}
        </GridTable>
      </RaisedCard>
    </FieldPanel>
  );
};
