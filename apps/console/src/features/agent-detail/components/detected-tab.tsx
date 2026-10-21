import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  Button,
  Callout,
  Checkbox,
  CodeBlock,
  EmptyState,
  FieldPanel,
  RaisedCard,
  Skeleton,
  StatusBadge,
  useToast,
} from "@/design-system";
import {
  type ServiceAgentDiscoveryItem,
  createAgentMonitorDefinition,
  getGetAgentDiscoveryQueryKey,
  useGetAgentDiscovery,
} from "@/orion-sdk";
import { formatDate } from "@/utils/date";
import {
  acceptHint,
  canAccept,
  groupItems,
  kindTitle,
  managedLabel,
  monitorDefinition,
} from "../detected-model";

const confidenceTone = (confidence?: string) =>
  confidence === "high" ? "up" : confidence === "medium" ? "degraded" : "unknown";

const Item = ({
  item,
  selectable,
  checked,
  onToggle,
}: {
  item: ServiceAgentDiscoveryItem;
  selectable: boolean;
  checked: boolean;
  onToggle: () => void;
}) => (
  <div className="grid min-h-[44px] grid-cols-[20px_minmax(0,1fr)_auto_auto] items-center gap-3 border-t border-line-soft px-3.5 py-2 text-[13px] first:border-t-0">
    {selectable ? (
      <Checkbox aria-label={`Select ${item.name}`} checked={checked} onCheckedChange={onToggle} />
    ) : (
      <span />
    )}
    <span className="min-w-0">
      <b className="block truncate font-medium">{item.name}</b>
      {item.detail && <small className="block truncate text-xs text-quiet">{item.detail}</small>}
      {!selectable && item.status !== "monitored" && !item.dismissed && (
        <small className="block truncate text-xs text-faint">{acceptHint(item)}</small>
      )}
    </span>
    <span className="flex items-center gap-1.5">
      {item.new && <StatusBadge status="info">New</StatusBadge>}
      {item.status === "monitored" ? (
        <StatusBadge status="up">{managedLabel(item.managed_in)}</StatusBadge>
      ) : item.dismissed ? (
        <StatusBadge status="stale">Dismissed</StatusBadge>
      ) : (
        <StatusBadge status="unknown" shape="none">
          Suggested
        </StatusBadge>
      )}
    </span>
    <StatusBadge status={confidenceTone(item.confidence)} shape="none">
      {item.confidence ?? "low"}
    </StatusBadge>
  </div>
);

/** "Detected on this server": what the agent found, with one click to monitor what Core can add. */
export const DetectedTab = ({ agentId }: { agentId: string }) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const query = useGetAgentDiscovery(agentId);
  const discovery = query.data?.discovery;
  const items = useMemo(() => discovery?.items ?? [], [discovery]);
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [accepting, setAccepting] = useState(false);

  // High-confidence suggestions that Core can add are ticked by default.
  useEffect(() => {
    setSelected(
      new Set(
        items
          .filter((item) => canAccept(item) && item.confidence === "high")
          .map((item) => item.id ?? ""),
      ),
    );
  }, [items]);

  const active = items.filter((item) => !item.dismissed);
  const dismissed = items.filter((item) => item.dismissed);
  const acceptable = active.filter(canAccept);
  const chosen = acceptable.filter((item) => selected.has(item.id ?? ""));
  const toggle = (id: string) =>
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const accept = async () => {
    setAccepting(true);
    let added = 0;
    let failed = 0;
    for (const item of chosen) {
      const data = monitorDefinition(item);
      if (!data) continue;
      try {
        await createAgentMonitorDefinition(agentId, data);
        added += 1;
      } catch {
        failed += 1;
      }
    }
    setAccepting(false);
    void queryClient.invalidateQueries({ queryKey: getGetAgentDiscoveryQueryKey(agentId) });
    toast.add({
      title:
        failed > 0
          ? `Added ${added}, ${failed} failed`
          : `Added ${added} ${added === 1 ? "monitor" : "monitors"}. They apply on the agent's next report.`,
    });
  };

  if (query.isPending) return <Skeleton className="h-48 w-full rounded-xl" />;
  if (query.isError)
    return (
      <EmptyState
        tone="error"
        title="Unable to load what was detected"
        action={
          <Button variant="secondary" onClick={() => void query.refetch()}>
            Retry
          </Button>
        }
      />
    );
  if (!discovery)
    return (
      <EmptyState
        className="rounded-xl border-solid bg-card"
        title="No scan reported yet"
        description="The agent sends what it finds after its first scan. Older agents do not send one, update the agent or add monitors by hand."
        action={
          <Link to="/monitors/new/agent">
            <Button variant="secondary">Create monitor</Button>
          </Link>
        }
      />
    );
  if (discovery.enabled === false)
    return (
      <EmptyState
        className="rounded-xl border-solid bg-card"
        title="Scanning is off"
        description="Host metrics keep working. Turn discovery on in the agent config (discovery.enabled), or create monitors by hand."
        action={
          <Link to="/monitors/new/agent">
            <Button variant="secondary">Create monitor</Button>
          </Link>
        }
      />
    );

  const problems = (discovery.kinds ?? []).filter((kind) => kind.state && kind.state !== "ok");
  return (
    <div className="flex flex-col gap-3.5">
      <div className="flex flex-wrap items-center justify-between gap-2 text-[13px] text-quiet">
        <span>
          <b className="font-medium text-ink">{discovery.things ?? 0} things found</b> ·{" "}
          {discovery.suggested ?? 0} suggested · {discovery.monitored ?? 0} monitored
          {(discovery.new_items ?? 0) > 0 && ` · ${discovery.new_items} new`}
        </span>
        <span>Last scan {formatDate(discovery.scanned_at, "MMM d, HH:mm", "unknown")}</span>
      </div>
      {problems.map((kind) => (
        <Callout key={kind.kind} tone="warn" lead={`Could not look at ${kind.kind}.`}>
          {kind.message}
          {kind.why ? ` ${kind.why}` : ""}
          {kind.fix && <CodeBlock className="mt-2">{kind.fix}</CodeBlock>}
        </Callout>
      ))}
      {active.length === 0 ? (
        <EmptyState
          className="rounded-xl border-solid bg-card"
          title="Nothing else detected"
          description="Host metrics are on. Run orion-agent discover --rescan on the server to scan again."
        />
      ) : (
        <>
          {groupItems(active).map(([kind, group]) => (
            <FieldPanel key={kind} title={kindTitle(kind)} aside={`${group.length}`}>
              <RaisedCard>
                {group.map((item) => (
                  <Item
                    key={item.id}
                    item={item}
                    selectable={canAccept(item)}
                    checked={selected.has(item.id ?? "")}
                    onToggle={() => toggle(item.id ?? "")}
                  />
                ))}
              </RaisedCard>
            </FieldPanel>
          ))}
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p className="max-w-[520px] text-[13px] text-quiet">
              Applies on the agent&apos;s next report, no restart. Services, containers, pm2 apps
              and log sources are accepted on the server with <code>orion-agent setup</code>.
            </p>
            <Button disabled={chosen.length === 0 || accepting} onClick={() => void accept()}>
              {accepting ? "Adding..." : `Accept selected (${chosen.length})`}
            </Button>
          </div>
        </>
      )}
      {dismissed.length > 0 && (
        <FieldPanel title="Dismissed" aside={`${dismissed.length}`}>
          <RaisedCard>
            {dismissed.map((item) => (
              <Item
                key={item.id}
                item={item}
                selectable={false}
                checked={false}
                onToggle={() => undefined}
              />
            ))}
          </RaisedCard>
        </FieldPanel>
      )}
    </div>
  );
};
