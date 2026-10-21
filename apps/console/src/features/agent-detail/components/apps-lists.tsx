import { Button, FieldPanel, GridRow, GridTable, RaisedCard, StatusBadge } from "@/design-system";
import type { ServiceAppResourceSeries } from "@/orion-sdk";
import { bytes, cpu, cpuWidth, dockerTitle, pair } from "../app-resources-model";

const Dash = () => <span className="text-faint">-</span>;

const Bar = ({ percent }: { percent: number }) => (
  <span className="h-1.5 w-16 shrink-0 overflow-hidden rounded-full bg-sub">
    <i className="block h-full rounded-full bg-mid" style={{ width: `${Math.max(percent, 1)}%` }} />
  </span>
);

const COLUMNS = "minmax(0,1.2fr) 80px 150px 150px 120px 120px 90px";

export const TopApps = ({ rows, total }: { rows: ServiceAppResourceSeries[]; total: number }) => {
  const maxMemory = Math.max(1, ...rows.map((row) => row.memory_bytes ?? 0));
  return (
    <FieldPanel
      title="Top apps"
      aside={`${rows.length} of ${total} · CPU is a share of one core · - not measured`}
    >
      <RaisedCard>
        <GridTable>
          <GridRow columns={COLUMNS} header>
            <span>App</span>
            <span>Kind</span>
            <span>CPU</span>
            <span>Memory</span>
            <span>Disk I/O</span>
            <span>Network</span>
            <span>Storage</span>
          </GridRow>
          {rows.map((row) => {
            const disk = pair(row.io_read_bytes_per_sec, row.io_write_bytes_per_sec);
            const net = pair(row.net_rx_bytes_per_sec, row.net_tx_bytes_per_sec);
            return (
              <GridRow key={row.key} columns={COLUMNS}>
                <b className="truncate font-medium">{row.name}</b>
                <span className="font-mono text-xs text-quiet">{row.kind}</span>
                <span className="flex items-center gap-2 tabular-nums">
                  {cpu(row.cpu_percent) ? (
                    <>
                      <Bar percent={cpuWidth(row.cpu_percent)} />
                      {cpu(row.cpu_percent)}
                    </>
                  ) : (
                    <Dash />
                  )}
                </span>
                <span className="flex items-center gap-2 tabular-nums">
                  {bytes(row.memory_bytes) ? (
                    <>
                      <Bar percent={((row.memory_bytes ?? 0) / maxMemory) * 100} />
                      {bytes(row.memory_bytes)}
                    </>
                  ) : (
                    <Dash />
                  )}
                </span>
                <span className="font-mono text-xs tabular-nums text-quiet">
                  {disk ?? <Dash />}
                </span>
                <span className="font-mono text-xs tabular-nums text-quiet">{net ?? <Dash />}</span>
                <span className="tabular-nums">{bytes(row.storage_bytes) ?? <Dash />}</span>
              </GridRow>
            );
          })}
        </GridTable>
      </RaisedCard>
    </FieldPanel>
  );
};

const SizeRows = ({
  rows,
  title,
}: {
  rows: ServiceAppResourceSeries[];
  title: (row: ServiceAppResourceSeries) => string;
}) => (
  <RaisedCard>
    {rows.map((row) => (
      <div
        key={row.key}
        className="grid min-h-[46px] grid-cols-[minmax(0,1fr)_130px] items-center gap-3 border-t border-line-soft px-3.5 py-1.5 text-[13px] first:border-t-0"
      >
        <span className="flex min-w-0 items-center gap-2.5">
          <b className="truncate font-medium">{title(row)}</b>
          {row.storage_capped && <StatusBadge status="degraded">Lower bound</StatusBadge>}
        </span>
        <span className="text-right tabular-nums">
          {row.storage_capped ? "≥ " : ""}
          {bytes(row.storage_bytes) ?? <Dash />}
        </span>
      </div>
    ))}
  </RaisedCard>
);

export const TrackedPaths = ({
  rows,
  canTrack,
  onTrack,
}: {
  rows: ServiceAppResourceSeries[];
  canTrack: boolean;
  onTrack: () => void;
}) => (
  <FieldPanel
    title="Tracked paths"
    aside={
      <span className="flex items-center gap-2.5">
        {rows.length > 0 && rows.length}
        <Button size="sm" variant="secondary" disabled={!canTrack} onClick={onTrack}>
          Track a directory
        </Button>
      </span>
    }
  >
    {rows.length === 0 ? (
      <RaisedCard>
        <div className="flex min-h-[46px] items-center justify-between gap-3 px-3.5 py-1.5 text-[13px]">
          <b className="font-medium">Nothing tracked yet</b>
          <span className="text-quiet">Track a directory to see its size here</span>
        </div>
      </RaisedCard>
    ) : (
      <SizeRows rows={rows} title={(row) => row.name ?? row.key ?? ""} />
    )}
  </FieldPanel>
);

export const DockerVolumes = ({ rows }: { rows: ServiceAppResourceSeries[] }) =>
  rows.length === 0 ? null : (
    <FieldPanel title="Docker volumes" aside={`${rows.length}`}>
      <SizeRows rows={rows} title={(row) => row.name ?? row.key ?? ""} />
    </FieldPanel>
  );

export const DockerDisk = ({ rows }: { rows: ServiceAppResourceSeries[] }) =>
  rows.length === 0 ? null : (
    <FieldPanel title="Docker images and build cache" aside="Refreshes every 5 minutes">
      <SizeRows rows={rows} title={(row) => dockerTitle(row.key)} />
    </FieldPanel>
  );
