import { useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { Button, EmptyState, Skeleton, Tabs, TabsList, TabsPanel, TabsTab } from "@/design-system";
import { useGetService } from "@/orion-sdk";
import { EditLinksDialog } from "./components/edit-links-dialog";
import { HealthBadge } from "./components/service-health";
import { AppsTable, LinkedPanel, MonitorsTable } from "./detail/linked-tables";
import { ServiceIncidentsTab } from "./detail/service-incidents-tab";
import { ServiceLogsTab } from "./detail/service-logs-tab";
import { ServiceOverviewTab } from "./detail/service-overview-tab";
import { ServiceResourcesTab } from "./detail/service-resources-tab";
import { ServiceSettingsTab } from "./detail/service-settings-tab";
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
  const [editing, setEditing] = useState(false);
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
  const monitors = service.monitors ?? [];
  const incidentId = firstActiveIncident(monitors);

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
          <Button size="sm" variant="secondary" onClick={() => setEditing(true)}>
            Edit links
          </Button>
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
        <TabsPanel value="overview">
          <ServiceOverviewTab service={service} onEditLinks={() => setEditing(true)} />
        </TabsPanel>
        <TabsPanel value="monitors" className="flex flex-col gap-4">
          {monitors.length > 0 ? (
            <LinkedPanel title="Monitors" action="Link monitor" onAction={() => setEditing(true)}>
              <MonitorsTable monitors={monitors} />
            </LinkedPanel>
          ) : (
            <EmptyState
              title="No monitors linked"
              description="Link a monitor so its incidents show this service's logs."
              action={
                <Button size="sm" onClick={() => setEditing(true)}>
                  Link a monitor
                </Button>
              }
            />
          )}
          {(service.apps ?? []).length > 0 && (
            <LinkedPanel title="Apps" action="Link app" onAction={() => setEditing(true)}>
              <AppsTable apps={service.apps ?? []} now={Date.now()} />
            </LinkedPanel>
          )}
        </TabsPanel>
        <TabsPanel value="logs">
          <ServiceLogsTab
            appIds={(service.apps ?? []).flatMap((app) => (app.id ? [app.id] : []))}
          />
        </TabsPanel>
        <TabsPanel value="resources">
          <ServiceResourcesTab resources={service.resources ?? []} />
        </TabsPanel>
        <TabsPanel value="incidents">
          <ServiceIncidentsTab monitorIds={monitors.flatMap((m) => (m.id ? [m.id] : []))} />
        </TabsPanel>
        <TabsPanel value="settings">
          <ServiceSettingsTab key={service.updated_at} service={service} />
        </TabsPanel>
      </Tabs>
      <EditLinksDialog service={service} open={editing} onOpenChange={setEditing} />
    </div>
  );
};
