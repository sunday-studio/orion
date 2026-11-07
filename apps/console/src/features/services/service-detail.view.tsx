import { Link, useParams, useSearchParams } from "react-router-dom";
import { Button, EmptyState, Skeleton, Tabs, TabsList, TabsPanel, TabsTab } from "@/design-system";
import { useGetService } from "@/orion-sdk";
import { HealthBadge } from "./components/service-health";
import {
  type ServiceTab,
  firstActiveIncident,
  serviceTabFromParam,
  summaryLine,
} from "./services.domain";

const TAB_LABELS: Record<ServiceTab, string> = {
  overview: "Overview",
  monitors: "Monitors",
  logs: "Logs",
  resources: "Resources",
  incidents: "Incidents",
  settings: "Settings",
};

export const ServiceDetailPage = () => {
  const { serviceId = "" } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const query = useGetService(serviceId);
  const service = query.data?.service;
  const tab = serviceTabFromParam(searchParams.get("tab"));

  if (query.isPending) return <Skeleton className="mt-10 h-40 w-full" />;
  if (query.isError || !service) {
    return (
      <EmptyState
        tone="error"
        title="Unable to load service"
        description="It may have been deleted, or Core is unreachable."
        action={
          <Link to="/services">
            <Button size="sm" variant="secondary">
              Back to services
            </Button>
          </Link>
        }
      />
    );
  }

  const setTab = (next: string) =>
    setSearchParams(
      (params) => {
        const copy = new URLSearchParams(params);
        if (next === "overview") copy.delete("tab");
        else copy.set("tab", next);
        return copy;
      },
      { replace: true },
    );
  const incidentId = firstActiveIncident(service.monitors ?? []);

  return (
    <div className="w-full pb-8">
      <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-2.5 pb-2 pt-7">
        <div className="min-w-0">
          <div className="text-[13.5px] leading-[1.4] text-quiet">
            <Link to="/services" className="hover:text-ink">
              Services
            </Link>{" "}
            / {service.name}
          </div>
          <h1 className="text-[28px] font-medium leading-10 tracking-[-0.025em]">{service.name}</h1>
          <div className="mt-1.5 flex flex-wrap items-center gap-2 text-[13.5px] text-quiet">
            <HealthBadge health={service.health} />
            <span>
              {[
                service.environment,
                summaryLine({
                  monitors: service.monitor_count,
                  apps: service.app_count,
                  resources: service.resource_count,
                }),
              ]
                .filter(Boolean)
                .join(" · ")}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-1.5">
          {incidentId && (
            <Link to={`/incidents/${incidentId}`}>
              <Button size="sm">Open incident</Button>
            </Link>
          )}
        </div>
      </div>
      <Tabs value={tab} onValueChange={setTab} className="mt-3 gap-3">
        <TabsList>
          {Object.entries(TAB_LABELS).map(([value, label]) => (
            <TabsTab key={value} value={value}>
              {label}
            </TabsTab>
          ))}
        </TabsList>
        {Object.keys(TAB_LABELS).map((value) => (
          <TabsPanel key={value} value={value} />
        ))}
      </Tabs>
    </div>
  );
};
