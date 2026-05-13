import { type ApiMonitorResponse, useGetAgentMonitors } from "@/orion-sdk";

const MonitorRow = ({ monitor }: { monitor: ApiMonitorResponse }) => {
  return (
    <div className="flex gap-2">
      <span className="font-medium">{monitor.name ?? monitor.id}</span>
      <span>{monitor.type}</span>
      <span>{monitor.health}</span>
      <span>{monitor.last_successful_report_at}</span>
      <span>{monitor.reporting_interval_seconds}</span>
      <span>{monitor.computed_health}</span>
      <span>{monitor.last_health_computation}</span>
      <span>{monitor.lifecycle}</span>
    </div>
  );
};

export const MonitorList = ({ agentId }: { agentId: string }) => {
  const monitorsResponse = useGetAgentMonitors(agentId);
  const monitors = monitorsResponse.data?.monitors ?? [];
  return (
    <div className="p-4">
      {monitors.map((monitor: ApiMonitorResponse) => (
        <MonitorRow key={monitor.id} monitor={monitor} />
      ))}
    </div>
  );
};
