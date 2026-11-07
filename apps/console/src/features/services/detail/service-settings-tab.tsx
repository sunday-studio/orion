import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Button, ConfirmDialog, Input, useToast } from "@/design-system";
import { type ServiceServiceDetail, deleteService, updateService } from "@/orion-sdk";
import { plural } from "../services.domain";
import { useRefreshServices } from "../use-service-mutations";

export const ServiceSettingsTab = ({ service }: { service: ServiceServiceDetail }) => {
  const serviceId = service.id ?? "";
  const navigate = useNavigate();
  const toast = useToast();
  const refresh = useRefreshServices();
  const [name, setName] = useState(service.name ?? "");
  const [environment, setEnvironment] = useState(service.environment ?? "");
  const [runbook, setRunbook] = useState(service.runbook_url ?? "");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [confirming, setConfirming] = useState(false);
  const dirty =
    name.trim() !== (service.name ?? "") ||
    environment.trim() !== (service.environment ?? "") ||
    runbook.trim() !== (service.runbook_url ?? "");

  const save = async () => {
    setPending(true);
    setError("");
    try {
      await updateService(serviceId, {
        name: name.trim(),
        environment: environment.trim(),
        runbook_url: runbook.trim(),
      });
      refresh(serviceId);
      toast.add({ title: "Service saved" });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Unable to save the service.");
    } finally {
      setPending(false);
    }
  };

  const remove = async () => {
    try {
      await deleteService(serviceId);
      refresh();
      toast.add({ title: `Deleted ${service.name}`, description: "Its links were cleared." });
      navigate("/services");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Unable to delete the service.");
      setConfirming(false);
    }
  };

  return (
    <div className="grid max-w-[520px] gap-4">
      <label className="grid gap-1.5 text-[12.5px] font-medium">
        Name
        <Input value={name} onChange={(event) => setName(event.target.value)} />
      </label>
      <label className="grid gap-1.5 text-[12.5px] font-medium">
        Environment
        <Input value={environment} onChange={(event) => setEnvironment(event.target.value)} />
      </label>
      <label className="grid gap-1.5 text-[12.5px] font-medium">
        Runbook link
        <Input
          placeholder="https://"
          value={runbook}
          onChange={(event) => setRunbook(event.target.value)}
        />
      </label>
      {error && (
        <p role="alert" className="text-[12.5px] text-[#be123c]">
          {error}
        </p>
      )}
      <div>
        <Button disabled={!dirty || pending || !name.trim()} onClick={() => void save()}>
          {pending ? "Saving..." : "Save"}
        </Button>
      </div>
      <div className="mt-4 border-t border-hairline pt-4">
        <h2 className="text-sm font-medium">Delete service</h2>
        <p className="mt-1 text-[13px] text-quiet">
          Only the grouping goes away. The {plural(service.monitor_count ?? 0, "monitor")},{" "}
          {plural(service.app_count ?? 0, "app")} and{" "}
          {plural(service.resource_count ?? 0, "resource")} stay as they are, with their history.
        </p>
        <Button variant="destructive" className="mt-3" onClick={() => setConfirming(true)}>
          Delete service
        </Button>
      </div>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={`Delete ${service.name}?`}
        description="The service is removed and its links are cleared. Monitors, apps, resources, logs and incidents are kept."
        confirmLabel="Delete service"
        onConfirm={() => void remove()}
      />
    </div>
  );
};
