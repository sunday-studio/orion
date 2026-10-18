import { Button, Callout, FieldPanel, RaisedCard, Skeleton, StatusBadge } from "@/design-system";
import { useGetSystemSettings } from "@/orion-sdk";
import { formatDate } from "@/utils/date";
import { SettingRow } from "../components-ui";

const size = (bytes?: number) =>
  (bytes ?? 0) >= 1_073_741_824
    ? `${((bytes ?? 0) / 1_073_741_824).toFixed(1)} GB`
    : `${Math.max(Math.round((bytes ?? 0) / 1_048_576), 1)} MB`;

const uptime = (seconds?: number) => {
  const s = seconds ?? 0;
  const days = Math.floor(s / 86400);
  const hours = Math.floor((s % 86400) / 3600);
  return days > 0 ? `${days}d ${hours}h` : `${hours}h ${Math.floor((s % 3600) / 60)}m`;
};

export const SystemTab = () => {
  const query = useGetSystemSettings({ query: { refetchInterval: 30_000 } });
  const data = query.data;
  if (query.isPending) return <Skeleton className="h-48 w-full rounded-xl" />;
  if (query.isError || !data) return <Callout tone="down">Unable to load system status.</Callout>;
  const worker = data.worker;
  const dot = worker?.healthy ? "#2a9f69" : worker?.status === "stale" ? "#f59e0b" : "#e5484d";
  return (
    <FieldPanel title="System" aside={`Core ${data.version ?? ""}`}>
      <RaisedCard>
        <div className="grid min-h-[38px] grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)_auto] items-center gap-3 px-3.5 py-2 text-[13px]">
          <b className="flex items-center gap-2.5 font-medium">
            <i className="size-2 rounded-full" style={{ background: dot }} />
            Core worker
          </b>
          <span className="truncate text-quiet">
            {worker?.status === "running"
              ? `Running, last tick ${formatDate(worker.last_tick_at, "HH:mm:ss")}`
              : worker?.last_error || (worker?.status ?? "Unknown")}
          </span>
          <StatusBadge
            status={worker?.healthy ? "up" : worker?.status === "stale" ? "degraded" : "down"}
          >
            {worker?.healthy ? "Healthy" : worker?.status === "stale" ? "Stale" : "Missing"}
          </StatusBadge>
        </div>
        <SettingRow
          label="Uptime"
          detail={`Up ${uptime(data.uptime_seconds)}, started ${formatDate(data.started_at, "MMM d, HH:mm")}`}
        />
        <SettingRow
          label="Database"
          detail={`${data.database?.engine ?? "SQLite"}, ${size(data.database?.size_bytes)}`}
        />
        <SettingRow label="Backups" detail={data.backups?.note}>
          {data.backups?.guide_url && (
            <a href={data.backups.guide_url} target="_blank" rel="noreferrer">
              <Button size="sm" variant="secondary">
                Open guide
              </Button>
            </a>
          )}
        </SettingRow>
      </RaisedCard>
    </FieldPanel>
  );
};
