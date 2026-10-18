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
  Select,
  Textarea,
} from "@/design-system";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import {
  getGetIncidentsQueryKey,
  getGetOverviewQueryKey,
  useDeclareIncident,
  useGetMonitors,
} from "@/orion-sdk";
import { cn } from "@/utils/cn";
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
  const [monitorId, setMonitorId] = useState("");
  const [summary, setSummary] = useState("");
  const [notify, setNotify] = useState(true);
  const [error, setError] = useState<string>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const declare = useDeclareIncident();
  const monitors = useGetMonitors({ limit: 100 }, { query: { enabled: open } });
  const monitorOptions = (monitors.data?.monitors ?? []).map((monitor) => ({
    value: monitor.id ?? "",
    label: monitor.name ?? monitor.id ?? "Monitor",
  }));

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
                  monitor_id: monitorId || undefined,
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
            <div
              role="radiogroup"
              aria-label="Severity"
              className="flex w-fit gap-0.5 rounded-lg bg-tray p-[3px]"
            >
              {severityOptions
                .slice()
                .reverse()
                .map((option) => (
                  <button
                    key={option.value}
                    type="button"
                    role="radio"
                    aria-checked={severity === option.value}
                    onClick={() => setSeverity(option.value)}
                    className={cn(
                      "rounded-md px-3 py-1 text-[13px] font-normal outline-none focus-visible:ring-2 focus-visible:ring-focus",
                      severity === option.value
                        ? "bg-chip font-medium shadow-[0_1px_2px_rgba(0,0,0,0.08)]"
                        : "text-mid",
                    )}
                  >
                    {option.label}
                  </button>
                ))}
            </div>
          </div>
          <div className="flex flex-col gap-1.5 text-[13px] font-medium">
            <span>Affected monitor or server</span>
            <Select
              aria-label="Affected monitor or server"
              className="w-full font-normal"
              options={monitorOptions}
              value={monitorId}
              onValueChange={setMonitorId}
            />
          </div>
          <label className="flex flex-col gap-1.5 text-[13px] font-medium">
            Summary
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
            <Button type="submit" disabled={!title.trim() || declare.isPending}>
              Declare incident
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
