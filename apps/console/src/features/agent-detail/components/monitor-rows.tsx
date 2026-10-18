import type { ApiMonitorResponse } from "@/orion-sdk";
import { Dot, StatusBadge } from "@/design-system";
import { healthDot } from "@/features/incidents/incident-format";
import { monitorHealth } from "../agent-detail.domain";

type MonitorWithDetail = ApiMonitorResponse & { status_detail?: string | null };

export const monitorTypeLabel = (type?: string) =>
  !type ? "-" : type === "http" ? "HTTP" : type.charAt(0).toUpperCase() + type.slice(1);

export const monitorStatusText = (monitor: MonitorWithDetail) => {
  const health = monitorHealth(monitor);
  return health.charAt(0).toUpperCase() + health.slice(1);
};

export const monitorDetail = (monitor: MonitorWithDetail) => monitor.status_detail ?? "-";

export const MONITOR_COLUMNS = "12px minmax(0,1.2fr) 100px 120px minmax(0,1.6fr)";

export const MonitorCells = ({ monitor }: { monitor: MonitorWithDetail }) => {
  const health = monitorHealth(monitor);
  return (
    <>
      <Dot color={healthDot(health === "stale" ? "unknown" : health)} />
      <b className="font-medium">{monitor.name ?? monitor.id}</b>
      <span className="text-faint">{monitorTypeLabel(monitor.type)}</span>
      <span>
        <StatusBadge status={health}>{monitorStatusText(monitor)}</StatusBadge>
      </span>
      <span className="text-faint">{monitorDetail(monitor)}</span>
    </>
  );
};
