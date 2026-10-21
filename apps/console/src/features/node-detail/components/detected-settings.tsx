import { FieldPanel, RaisedCard, StatusBadge, Switch } from "@/design-system";
import type { ServiceNodeDiscoveryKind } from "@/orion-sdk";
import { kindScan, kindTitle } from "../detected-model";

const DETAIL: Record<string, string> = {
  service: "systemd units",
  container: "Docker containers",
  pm2: "pm2 apps",
  port: "Listening TCP ports",
  disk: "Mounts and usage",
  log: "Common log files",
  storage: "Data directories",
};

const isOff = (kind: ServiceNodeDiscoveryKind) => kindScan(kind) === "off";

const stateBadge = (kind: ServiceNodeDiscoveryKind) => {
  const scan = kindScan(kind);
  if (scan === "off") return { tone: "stale" as const, text: "Not scanned" };
  if (scan === "absent") return { tone: "unknown" as const, text: "Nothing to scan" };
  if (scan === "problem") return { tone: "degraded" as const, text: "Limited" };
  return { tone: "up" as const, text: "Scanned" };
};

const Row = ({ children }: { children: React.ReactNode }) => (
  <div className="grid min-h-[46px] grid-cols-[minmax(0,1fr)_150px_40px] items-center gap-3 border-t border-line-soft px-3.5 py-1.5 text-[13px] first:border-t-0">
    {children}
  </div>
);

/** Scan settings: scanning on or off and what to look at. Changes are queued for the node. */
export const DetectedSettings = ({
  enabled,
  kinds,
  canAct,
  pending,
  onScanning,
  onKind,
}: {
  enabled: boolean;
  kinds: ServiceNodeDiscoveryKind[];
  canAct: boolean;
  /** Kinds (or "scanning") with a change waiting for the node. */
  pending: string[];
  onScanning: (enabled: boolean) => void;
  onKind: (kind: string, enabled: boolean) => void;
}) => (
  <div className="flex flex-col gap-3.5">
    <FieldPanel title="Scanning">
      <RaisedCard>
        <div className="flex min-h-[46px] items-center justify-between gap-3 px-3.5 py-1.5 text-[13px]">
          <span className="flex min-w-0 items-center gap-2.5">
            <b className="shrink-0 font-medium">Scan this server</b>
            <span className="truncate text-xs text-faint">
              Host metrics keep working when this is off
            </span>
          </span>
          <span className="flex items-center gap-2">
            {pending.includes("scanning") && <StatusBadge status="degraded">Pending</StatusBadge>}
            <Switch
              aria-label="Scan this server"
              checked={enabled}
              disabled={!canAct}
              onCheckedChange={onScanning}
            />
          </span>
        </div>
      </RaisedCard>
    </FieldPanel>
    <FieldPanel
      title="What to look at"
      aside={`${kinds.filter((kind) => !isOff(kind)).length} of ${kinds.length}`}
    >
      <RaisedCard>
        {kinds.map((kind) => {
          const badge = stateBadge(kind);
          return (
            <Row key={kind.kind}>
              <span className="flex min-w-0 items-center gap-2.5">
                <b className="shrink-0 font-medium">{kindTitle(kind.kind)}</b>
                <span className="truncate text-xs text-faint">
                  {kind.message || DETAIL[kind.kind ?? ""] || ""}
                </span>
              </span>
              {pending.includes(kind.kind ?? "") ? (
                <StatusBadge status="degraded">Pending</StatusBadge>
              ) : (
                <StatusBadge status={badge.tone}>{badge.text}</StatusBadge>
              )}
              <Switch
                aria-label={kindTitle(kind.kind)}
                checked={!isOff(kind)}
                disabled={!canAct || !enabled}
                onCheckedChange={(next) => onKind(kind.kind ?? "", next)}
              />
            </Row>
          );
        })}
      </RaisedCard>
      {!canAct && (
        <p className="mt-2 text-[13px] text-mid">
          This node cannot take requests from the Console. Update the node, or on the server run{" "}
          <code>orion-node discover --rescan</code>.
        </p>
      )}
    </FieldPanel>
  </div>
);
