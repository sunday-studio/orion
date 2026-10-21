import type { ApiMonitorResponse } from "@/orion-sdk";
import { StatusBadge } from "@/design-system";
import {} from "@/features/incidents/incident-format";
import { monitorHealth } from "../node-detail.domain";

type MonitorWithDetail = ApiMonitorResponse;

const ACRONYMS = new Set(["http", "https", "tcp", "udp", "dns", "tls", "icmp", "ssl"]);

/** "http-healthcheck" -> "HTTP", "docker-container" -> "Docker": the short family name. */
export const monitorTypeLabel = (type?: string) => {
  if (!type) return "-";
  const family = type.toLowerCase().split(/[-_ ]/)[0] ?? type;
  return ACRONYMS.has(family)
    ? family.toUpperCase()
    : family.charAt(0).toUpperCase() + family.slice(1);
};

export const monitorStatusText = (monitor: MonitorWithDetail) => {
  if (monitor.status_text) return monitor.status_text;
  const health = monitorHealth(monitor);
  return health.charAt(0).toUpperCase() + health.slice(1);
};

export const monitorDetail = (monitor: MonitorWithDetail) => monitor.status_detail ?? "-";

export const MONITOR_COLUMNS = "minmax(0,1.6fr) 76px 112px minmax(0,1.2fr)";

export const MonitorCells = ({ monitor }: { monitor: MonitorWithDetail }) => {
  const health = monitorHealth(monitor);
  return (
    <>
      <b className="font-medium">{monitor.name ?? monitor.id}</b>
      <span className="text-faint">{monitorTypeLabel(monitor.type)}</span>
      <span>
        <StatusBadge status={health}>{monitorStatusText(monitor)}</StatusBadge>
      </span>
      <span className="text-faint">{monitorDetail(monitor)}</span>
    </>
  );
};
