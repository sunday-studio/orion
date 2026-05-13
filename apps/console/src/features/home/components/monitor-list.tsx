import { type ApiMonitorResponse, useGetAgentMonitors } from "@/orion-sdk";
import { Separator } from "@/components/ui/separator";
import { Fragment } from "react/jsx-runtime";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";

const MonitorRow = ({ monitor }: { monitor: ApiMonitorResponse }) => {
  const health = monitor.health ?? monitor.computed_health ?? "unknown";

  return (
    <div className="flex items-center gap-3 py-2">
      <span className="font-medium">{monitor.name ?? monitor.id}</span>
      <span>{health}</span>
      <span>{monitor.type}</span>
      <span>last success {formatDate(monitor.last_successful_report_at, DATE_TIME_FORMAT)}</span>
      {monitor.lifecycle && monitor.lifecycle !== "active" && <span>{monitor.lifecycle}</span>}
    </div>
  );
};

export const MonitorList = ({ agentId }: { agentId: string }) => {
  const monitorsResponse = useGetAgentMonitors(agentId);
  const monitors = monitorsResponse.data?.monitors ?? [];
  return (
    <div className="pl-6">
      {monitors.map((monitor: ApiMonitorResponse, index: number) => (
        <Fragment key={monitor.id}>
          <MonitorRow monitor={monitor} />
          {index < monitors.length - 1 && <Separator />}
        </Fragment>
      ))}
    </div>
  );
};
