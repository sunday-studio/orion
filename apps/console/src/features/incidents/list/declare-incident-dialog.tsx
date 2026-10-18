import { useState } from "react";
import {
  Button,
  Callout,
  Checkbox,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Segmented,
  Select,
  Textarea,
} from "@/design-system";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import {
  getGetIncidentsQueryKey,
  getGetOverviewQueryKey,
  useDeclareIncident,
  useGetAgents,
  useGetMonitors,
} from "@/orion-sdk";
import { severityOptions } from "../incident-format";

export const DeclareIncidentDialog = ({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) => {
  const [title, setTitle] = useState("");
  const [severity, setSeverity] = useState("high");
  const [target, setTarget] = useState("");
  const [summary, setSummary] = useState("");
  const [notify, setNotify] = useState(true);
  const [error, setError] = useState<string>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const declare = useDeclareIncident();
  const monitors = useGetMonitors({ limit: 100 }, { query: { enabled: open } });
  const agents = useGetAgents({ limit: 100 }, { query: { enabled: open } });
  const targetOptions = [
    ...(monitors.data?.monitors ?? []).map((monitor) => ({
      value: `monitor:${monitor.id ?? ""}`,
      label: `${monitor.name ?? monitor.id ?? "Monitor"}${monitor.agent_name ? ` on ${monitor.agent_name}` : ""}`,
    })),
    ...(agents.data?.agents ?? []).map((agent) => ({
      value: `agent:${agent.id ?? ""}`,
      label: `${agent.name ?? agent.id ?? "Server"} (server)`,
    })),
  ];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[520px] gap-4 rounded-2xl p-6">
        <DialogHeader>
          <DialogTitle className="text-lg font-medium">Declare incident</DialogTitle>
        </DialogHeader>
        <p className="-mt-2 text-sm text-mid">
          Open one by hand when Orion has not noticed something.
        </p>
        <form
          className="flex flex-col gap-3.5"
          onSubmit={(event) => {
            event.preventDefault();
            setError(undefined);
            declare.mutate(
              {
                data: {
                  title: title.trim(),
                  severity: severity as "low" | "medium" | "high" | "critical",
                  monitor_id: target.startsWith("monitor:") ? target.slice(8) : undefined,
                  agent_id: target.startsWith("agent:") ? target.slice(6) : undefined,
                  summary: summary.trim() || undefined,
                  notify,
                },
              },
              {
                onSuccess: (result) => {
                  void queryClient.invalidateQueries({ queryKey: getGetIncidentsQueryKey() });
                  void queryClient.invalidateQueries({ queryKey: getGetOverviewQueryKey() });
                  onOpenChange(false);
                  if (result.incident?.id) navigate(`/incidents/${result.incident.id}`);
                },
                onError: (failure) =>
                  setError(
                    (failure as { status?: number }).status === 409
                      ? "That monitor already has an active incident."
                      : "Unable to declare the incident. Try again.",
                  ),
              },
            );
          }}
        >
          <label className="flex flex-col gap-1.5 text-[13px] font-medium">
            Title
            <Input
              className="font-normal"
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              placeholder="Checkout is slow for EU customers"
            />
          </label>
          <div className="flex flex-col gap-1.5 text-[13px] font-medium">
            Severity
            <Segmented
              aria-label="Severity"
              options={severityOptions
                .slice()
                .reverse()
                .map((option) => ({ value: option.value, label: option.label }))}
              value={severity}
              onValueChange={setSeverity}
            />
          </div>
          <div className="flex flex-col gap-1.5 text-[13px] font-medium">
            <span>Monitor or server</span>
            <Select
              aria-label="Monitor or server"
              className="w-full font-normal"
              options={targetOptions}
              value={target}
              onValueChange={setTarget}
            />
            <span className="text-[13px] font-normal text-quiet">
              A monitor can have only one active incident.
            </span>
          </div>
          <label className="flex flex-col gap-1.5 text-[13px] font-medium">
            Summary, optional
            <Textarea
              className="font-normal"
              value={summary}
              onChange={(event) => setSummary(event.target.value)}
              placeholder="What are you seeing?"
            />
          </label>
          <label className="flex items-center gap-2 text-[13px]">
            <Checkbox checked={notify} onCheckedChange={(next) => setNotify(Boolean(next))} />
            Notify channels using the matching rule
          </label>
          {error && <Callout tone="warn">{error}</Callout>}
          <DialogFooter>
            <Button variant="secondary" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!title.trim() || !target || declare.isPending}>
              Declare incident
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
