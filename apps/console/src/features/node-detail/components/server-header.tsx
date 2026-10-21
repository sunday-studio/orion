import { useState } from "react";
import { Button, StatusBadge, useToast } from "@/design-system";
import { MaintenanceDialog } from "./maintenance-dialog";
import {
  type ServerRow,
  platformLabel,
  serverState,
  statusText,
} from "@/features/nodes/server-format";
import { formatSpan } from "@/features/overview/overview-time";
import { getGetNodeQueryKey, getGetNodesQueryKey, useSetMaintenanceMode } from "@/orion-sdk";
import { useQueryClient } from "@tanstack/react-query";

type Props = {
  node: ServerRow;
  nodeVersion?: string;
  now: number;
  onEdit: () => void;
  onToken: () => void;
};

/** Shared maintenance toggle: enabling lasts 24 hours, ending it is immediate. */
export const useServerMaintenance = (node: ServerRow) => {
  const queryClient = useQueryClient();
  const toast = useToast();
  const mutation = useSetMaintenanceMode({
    mutation: {
      onSuccess: (_data, variables) => {
        void queryClient.invalidateQueries({ queryKey: getGetNodeQueryKey(variables.nodeId) });
        void queryClient.invalidateQueries({ queryKey: getGetNodesQueryKey() });
      },
      onError: () => toast.add({ title: "Unable to change maintenance mode" }),
    },
  });
  return {
    inMaintenance: Boolean(node.maintenance_mode),
    pending: mutation.isPending,
    set: (enabled: boolean, durationSeconds = 86_400) =>
      mutation.mutate({
        nodeId: node.id ?? "",
        data: enabled
          ? { maintenance_mode: true, duration_seconds: durationSeconds }
          : { maintenance_mode: false },
      }),
  };
};

/** Name, state badge and platform line, with Edit / Maintenance / Token actions. */
export const ServerHeader = ({ node, nodeVersion, now, onEdit, onToken }: Props) => {
  const maintenance = useServerMaintenance(node);
  const [confirm, setConfirm] = useState(false);
  const state = serverState(node);
  const parts = [
    platformLabel(node),
    nodeVersion ? `Node ${nodeVersion}` : undefined,
    state === "up" && node.uptime_seconds
      ? `up ${formatSpan(0, node.uptime_seconds * 1000, false, true).split(" ")[0]}`
      : undefined,
  ].filter(Boolean);
  return (
    <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-2.5 pb-2 pt-7">
      <div className="min-w-0">
        <h1 className="text-[28px] font-medium leading-10 tracking-[-0.025em]">
          {node.name ?? node.id ?? "Unknown server"}
        </h1>
        <div className="mt-1.5 flex items-center gap-2 text-[13.5px] text-quiet">
          <StatusBadge status={state}>{statusText(node, now)}</StatusBadge>
          <span>{parts.join(", ")}</span>
        </div>
      </div>
      <div className="flex items-center gap-1.5">
        <Button size="sm" variant="secondary" onClick={onEdit}>
          Edit
        </Button>
        <Button
          size="sm"
          variant="secondary"
          disabled={maintenance.pending}
          onClick={() => (maintenance.inMaintenance ? maintenance.set(false) : setConfirm(true))}
        >
          {maintenance.inMaintenance ? "End maintenance" : "Maintenance"}
        </Button>
        <Button size="sm" variant="secondary" onClick={onToken}>
          Token
        </Button>
      </div>
      <MaintenanceDialog
        open={confirm}
        pending={maintenance.pending}
        onOpenChange={setConfirm}
        onStart={(seconds) => {
          maintenance.set(true, seconds);
          setConfirm(false);
        }}
      />
    </div>
  );
};
