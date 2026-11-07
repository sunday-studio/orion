import { useState } from "react";
import {
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Select,
  Skeleton,
  useToast,
} from "@/design-system";
import {
  createService,
  linkServiceItem,
  useListServiceSuggestions,
  useListServices,
} from "@/orion-sdk";
import { cn } from "@/utils/cn";
import { acceptPlan } from "../link-runner";
import { defaultMonitorChoice, monitorLinkChoices } from "../services.domain";
import { useRefreshMonitor, useRefreshServices } from "../use-service-mutations";

/** Link one monitor to a service. Suggestions come with their reason and are never applied unasked. */
export const MonitorLinkDialog = ({
  monitor,
  open,
  onOpenChange,
}: {
  monitor: { id: string; name?: string; service_id?: string };
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) => {
  const suggestions = useListServiceSuggestions(undefined, { query: { enabled: open } });
  const services = useListServices({ query: { enabled: open } });
  const choices = monitorLinkChoices(
    suggestions.data?.suggestions ?? [],
    monitor,
    services.data?.services ?? [],
  );
  const [picked, setPicked] = useState("");
  const [other, setOther] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const refreshServices = useRefreshServices();
  const refreshMonitor = useRefreshMonitor();
  const toast = useToast();

  const choiceKey = other ? `svc:${other}` : picked || defaultMonitorChoice(choices);
  const chosen = other
    ? { serviceId: other, name: services.data?.services?.find((s) => s.id === other)?.name }
    : choices.find((choice) => choice.key === choiceKey);
  const options = (services.data?.services ?? [])
    .filter((service) => service.id !== monitor.service_id)
    .map((service) => ({ value: service.id ?? "", label: service.name ?? service.id ?? "" }));

  const submit = async () => {
    if (!chosen) return;
    setPending(true);
    setError("");
    const result = await acceptPlan(
      {
        serviceId: "serviceId" in chosen ? chosen.serviceId : undefined,
        create: "create" in chosen ? chosen.create : undefined,
        refs: [{ kind: "monitor", id: monitor.id }],
      },
      { create: createService, link: linkServiceItem },
    );
    setPending(false);
    refreshServices(result.serviceId);
    refreshMonitor(monitor.id);
    const failure = result.error ?? result.outcome.failed[0]?.message;
    if (failure) {
      setError(failure);
      return;
    }
    toast.add({ title: `Linked ${monitor.name ?? "monitor"} to ${chosen.name ?? "the service"}` });
    onOpenChange(false);
    setPicked("");
    setOther("");
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[520px] gap-3.5 rounded-2xl p-5">
        <DialogHeader>
          <DialogTitle className="text-[16px]">
            Link {monitor.name ?? "this monitor"} to a service
          </DialogTitle>
          <DialogDescription>
            Its incidents will show that service's logs and resource use.
          </DialogDescription>
        </DialogHeader>
        {options.length > 0 && (
          <label className="grid gap-1.5 text-[12.5px] font-medium">
            Service
            <Select
              aria-label="Service"
              className="w-full font-normal"
              options={[{ value: "", label: "Choose a service" }, ...options]}
              value={other}
              onValueChange={setOther}
            />
          </label>
        )}
        <div className="grid gap-2 text-[12.5px] font-medium">
          Looks like
          {suggestions.isPending ? (
            <Skeleton className="h-20 w-full" />
          ) : (
            <ul role="radiogroup" aria-label="Suggested services" className="grid gap-2">
              {choices.map((choice) => {
                const on = !other && choice.key === choiceKey;
                return (
                  <li key={choice.key}>
                    <button
                      type="button"
                      role="radio"
                      aria-checked={on}
                      onClick={() => {
                        setOther("");
                        setPicked(choice.key);
                      }}
                      className={cn(
                        "grid w-full grid-cols-[20px_minmax(0,1fr)_auto] items-start gap-2.5 rounded-lg border p-3 text-left font-normal",
                        on ? "border-brand bg-sub" : "border-hairline",
                      )}
                    >
                      <span
                        aria-hidden
                        className={cn(
                          "mt-0.5 size-4 rounded-full border-[1.5px]",
                          on
                            ? "border-brand bg-brand/80 ring-2 ring-inset ring-card"
                            : "border-faint",
                        )}
                      />
                      <span className="min-w-0">
                        <b className="text-[13px] font-medium">{choice.name}</b>{" "}
                        {choice.detail && <span className="text-faint">{choice.detail}</span>}
                        {choice.reasons.length > 0 && (
                          <span className="mt-2 flex flex-wrap gap-1.5">
                            {choice.reasons.map((reason) => (
                              <span
                                key={reason}
                                className="rounded bg-tray px-1.5 py-0.5 text-[11.5px]"
                              >
                                {reason}
                              </span>
                            ))}
                          </span>
                        )}
                      </span>
                      {choice.likelihood && (
                        <Badge tone={choice.likelihood === "likely" ? "up" : "unknown"}>
                          {choice.likelihood === "likely" ? "Likely" : "Maybe"}
                        </Badge>
                      )}
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
        {error && (
          <p role="alert" className="text-[12.5px] text-[#be123c]">
            {error}
          </p>
        )}
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button disabled={pending || !chosen} onClick={() => void submit()}>
            {pending ? "Linking..." : "Link monitor"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
