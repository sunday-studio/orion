import { useState } from "react";
import { Link } from "react-router-dom";
import { Button, FieldPanel, RaisedCard, useToast } from "@/design-system";
import { unlinkServiceItem, useListServices } from "@/orion-sdk";
import { serviceHref } from "../services.domain";
import { useRefreshMonitor, useRefreshServices } from "../use-service-mutations";
import { MonitorLinkDialog } from "./monitor-link-dialog";

/** Name of the service a thing is linked to, from the cached list. */
export const useServiceName = (serviceId?: string) => {
  const services = useListServices({ query: { enabled: Boolean(serviceId) } });
  return services.data?.services?.find((service) => service.id === serviceId)?.name;
};

/** Service chip for a monitor's header: "service checkout", linking to the service page. */
export const MonitorServiceChip = ({ serviceId }: { serviceId?: string }) => {
  const name = useServiceName(serviceId);
  if (!serviceId) return null;
  return (
    <>
      {" · service "}
      <Link to={serviceHref(serviceId)} className="underline hover:text-ink">
        {name ?? "linked"}
      </Link>
    </>
  );
};

/** Configure page section: which service this monitor belongs to, with link and unlink. */
export const MonitorServiceSection = ({
  monitor,
}: {
  monitor: { id: string; name?: string; service_id?: string; system?: boolean };
}) => {
  const [linking, setLinking] = useState(false);
  const [error, setError] = useState("");
  const name = useServiceName(monitor.service_id);
  const refreshServices = useRefreshServices();
  const refreshMonitor = useRefreshMonitor();
  const toast = useToast();
  if (monitor.system) return null;

  const unlink = async () => {
    if (!monitor.service_id) return;
    setError("");
    try {
      await unlinkServiceItem(monitor.service_id, { kind: "monitor", target_id: monitor.id });
      refreshServices(monitor.service_id);
      refreshMonitor(monitor.id);
      toast.add({ title: "Unlinked", description: "Logs and history are untouched." });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Unable to unlink.");
    }
  };

  return (
    <FieldPanel title="Service">
      <RaisedCard className="flex items-center gap-3 px-3.5 py-3">
        <i
          aria-hidden
          className={`block size-2 rounded-full ${monitor.service_id ? "bg-(--color-dot-up)" : "bg-faint"}`}
        />
        <div className="min-w-0 flex-1">
          {monitor.service_id ? (
            <Link
              to={serviceHref(monitor.service_id)}
              className="text-[13px] font-medium hover:underline"
            >
              {name ?? "Linked service"}
            </Link>
          ) : (
            <b className="text-[13px] font-medium">Not linked</b>
          )}
          <p className="text-xs text-quiet">
            {monitor.service_id
              ? "Incidents here show this service's app logs."
              : "Incidents here won't show app logs."}
          </p>
          {error && (
            <p role="alert" className="text-xs text-(--tone-down-fg)">
              {error}
            </p>
          )}
        </div>
        {monitor.service_id && (
          <Button size="sm" variant="ghost" onClick={() => void unlink()}>
            Unlink
          </Button>
        )}
        <Button size="sm" variant="secondary" onClick={() => setLinking(true)}>
          {monitor.service_id ? "Change service" : "Link a service"}
        </Button>
      </RaisedCard>
      <MonitorLinkDialog monitor={monitor} open={linking} onOpenChange={setLinking} />
    </FieldPanel>
  );
};
