import { useState } from "react";
import { Button, ConfirmDialog, StatusBadge, useToast } from "@/design-system";
import {
  type ServerRow,
  platformLabel,
  serverState,
  statusText,
} from "@/features/agents/server-format";
import { formatSpan } from "@/features/overview/overview-time";
import { getGetAgentQueryKey, getGetAgentsQueryKey, useSetMaintenanceMode } from "@/orion-sdk";
import { useQueryClient } from "@tanstack/react-query";

type Props = {
  agent: ServerRow;
  agentVersion?: string;
  now: number;
  onEdit: () => void;
  onToken: () => void;
};

/** Shared maintenance toggle: enabling lasts 24 hours, ending it is immediate. */
export const useServerMaintenance = (agent: ServerRow) => {
  const queryClient = useQueryClient();
  const toast = useToast();
  const mutation = useSetMaintenanceMode({
    mutation: {
      onSuccess: (_data, variables) => {
        void queryClient.invalidateQueries({ queryKey: getGetAgentQueryKey(variables.agentId) });
        void queryClient.invalidateQueries({ queryKey: getGetAgentsQueryKey() });
      },
      onError: () => toast.add({ title: "Unable to change maintenance mode" }),
    },
  });
  return {
    inMaintenance: Boolean(agent.maintenance_mode),
    pending: mutation.isPending,
    set: (enabled: boolean) =>
      mutation.mutate({
        agentId: agent.id ?? "",
        data: enabled
          ? { maintenance_mode: true, duration_seconds: 86_400 }
          : { maintenance_mode: false },
      }),
  };
};

/** Name, state badge and platform line, with Edit / Maintenance / Token actions. */
export const ServerHeader = ({ agent, agentVersion, now, onEdit, onToken }: Props) => {
  const maintenance = useServerMaintenance(agent);
  const [confirm, setConfirm] = useState(false);
  const state = serverState(agent);
  const parts = [
    platformLabel(agent),
    agentVersion ? `agent ${agentVersion}` : undefined,
    state === "up" && agent.uptime_seconds
      ? `up ${formatSpan(0, agent.uptime_seconds * 1000, false, true).split(" ")[0]}`
      : undefined,
  ].filter(Boolean);
  return (
    <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-2.5 pb-2 pt-7">
      <div className="min-w-0">
        <h1 className="text-[28px] font-medium leading-10 tracking-[-0.025em]">
          {agent.name ?? agent.id ?? "Unknown server"}
        </h1>
        <div className="mt-1.5 flex items-center gap-2 text-[13.5px] text-quiet">
          <StatusBadge status={state}>{statusText(agent, now)}</StatusBadge>
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
      <ConfirmDialog
        open={confirm}
        onOpenChange={setConfirm}
        title={`Put ${agent.name ?? "this server"} in maintenance?`}
        description="Alerts for this server and its monitors pause for 24 hours, or until you end maintenance."
        confirmLabel="Start maintenance"
        tone="default"
        onConfirm={() => maintenance.set(true)}
      />
    </div>
  );
};
