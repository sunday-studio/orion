import { type ApiMonitorResponse, useGetAgentMonitors } from "@/orion-sdk";
import { Separator } from "@/components/ui/separator";
import { Fragment } from "react/jsx-runtime";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { Link } from "react-router-dom";

const MonitorRow = ({ monitor }: { monitor: ApiMonitorResponse }) => {
  const health = monitor.health ?? monitor.computed_health ?? "unknown";

  return (
    <div className="grid grid-cols-[minmax(0,1fr)_5rem] items-center gap-3 py-2 text-sm sm:grid-cols-[minmax(0,1.4fr)_7rem_7rem_minmax(0,1fr)]">
      <Link to={`/monitors/${monitor.id}`} className="truncate font-medium hover:text-neutral-600">
        {monitor.name ?? monitor.id}
      </Link>
      <span>{health}</span>
      <span className="hidden sm:inline">{monitor.type ?? "unknown"}</span>
      <span className="truncate text-neutral-600">
        last success {formatDate(monitor.last_successful_report_at, DATE_TIME_FORMAT)}
      </span>
    </div>
  );
};

export const MonitorList = ({ agentId }: { agentId: string }) => {
  const monitorsResponse = useGetAgentMonitors(agentId);
  const monitors = monitorsResponse.data?.monitors ?? [];

  if (monitorsResponse.isLoading) {
    return <div className="py-2 pl-6 text-sm text-neutral-600">Loading monitors...</div>;
  }

  if (monitorsResponse.error) {
    return <div className="py-2 pl-6 text-sm">Unable to load monitors.</div>;
  }

  if (monitors.length === 0) {
    return <div className="py-2 pl-6 text-sm text-neutral-600">No monitors registered.</div>;
  }

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
