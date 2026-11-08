import { useState } from "react";
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  useToast,
} from "@/design-system";
import { createService, linkServiceItem } from "@/orion-sdk";
import type { ServiceServiceLinkRef } from "@/orion-sdk";
import { acceptPlan } from "../link-runner";
import { canCreateService, plural } from "../services.domain";
import { useRefreshServices } from "../use-service-mutations";
import { LinkPicker } from "./link-picker";

/** New service: name it, and optionally link what it is made of. Linking can wait. */
export const ServiceFormDialog = ({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated?: (serviceId: string) => void;
}) => {
  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("");
  const [monitors, setMonitors] = useState<ServiceServiceLinkRef[]>([]);
  const [apps, setApps] = useState<ServiceServiceLinkRef[]>([]);
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  const refresh = useRefreshServices();
  const toast = useToast();

  const reset = () => {
    setName("");
    setEnvironment("");
    setMonitors([]);
    setApps([]);
    setError("");
  };

  const submit = async () => {
    setPending(true);
    setError("");
    const result = await acceptPlan(
      {
        create: {
          name: name.trim(),
          environment: environment.trim() || undefined,
        },
        refs: [...monitors, ...apps],
      },
      { create: (input) => createService(input), link: linkServiceItem },
    );
    setPending(false);
    refresh(result.serviceId);
    if (result.error || !result.serviceId) {
      setError(result.error ?? "Unable to create the service.");
      return;
    }
    const failed = result.outcome.failed.length;
    toast.add({
      title: `Created ${name.trim()}`,
      description: failed > 0 ? `${plural(failed, "link")} could not be added.` : undefined,
    });
    onOpenChange(false);
    reset();
    onCreated?.(result.serviceId);
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next);
        if (!next) reset();
      }}
    >
      <DialogContent className="max-h-[calc(100vh-4rem)] max-w-[520px] gap-3.5 overflow-y-auto rounded-2xl p-5">
        <DialogHeader>
          <DialogTitle className="text-[16px]">New service</DialogTitle>
          <DialogDescription>Name the thing you run. You can add the rest later.</DialogDescription>
        </DialogHeader>
        <label className="grid gap-1.5 text-[12.5px] font-medium">
          Name
          <Input value={name} onChange={(event) => setName(event.target.value)} autoFocus />
        </label>
        <div className="grid grid-cols-2 gap-3">
          <label className="grid gap-1.5 text-[12.5px] font-medium">
            Environment
            <Input
              placeholder="production"
              value={environment}
              onChange={(event) => setEnvironment(event.target.value)}
            />
          </label>
        </div>
        <div className="grid gap-1.5 text-[12.5px] font-medium">
          Monitors
          <LinkPicker kind="monitor" onChange={setMonitors} />
        </div>
        <div className="grid gap-1.5 text-[12.5px] font-medium">
          Apps
          <LinkPicker kind="app" onChange={setApps} />
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
          <Button disabled={pending || !canCreateService(name)} onClick={() => void submit()}>
            {pending ? "Creating..." : "Create service"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
