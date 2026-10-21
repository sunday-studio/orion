import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  Button,
  Callout,
  CodeBlock,
  EmptyState,
  FieldPanel,
  RaisedCard,
  Skeleton,
  useToast,
} from "@/design-system";
import {
  getGetAgentDiscoveryQueryKey,
  useAcceptDiscoveredItems,
  useGetAgentDiscovery,
} from "@/orion-sdk";
import { formatDate } from "@/utils/date";
import { canAccept, groupItems, kindTitle } from "../detected-model";
import { useDiscoveryRequest } from "./detected-actions";
import { DetectedItem } from "./detected-item";
import { DetectedRequests } from "./detected-requests";
import { DetectedSettings } from "./detected-settings";

/** "Detected on this server": what the agent found, accept it, dismiss it or change what is scanned. */
export const DetectedTab = ({ agentId }: { agentId: string }) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const query = useGetAgentDiscovery(agentId);
  const requests = useDiscoveryRequest(agentId);
  const discovery = query.data?.discovery;
  const items = useMemo(() => discovery?.items ?? [], [discovery]);
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const canAct = Boolean(discovery?.capabilities?.includes("requests"));
  const accept = useAcceptDiscoveredItems({
    mutation: {
      onSuccess: (result) => {
        void queryClient.invalidateQueries({ queryKey: getGetAgentDiscoveryQueryKey(agentId) });
        const created = result.created?.length ?? 0;
        const skipped = result.skipped?.length ?? 0;
        toast.add({
          title: `Added ${created} ${created === 1 ? "monitor" : "monitors"}${
            skipped > 0 ? `, ${skipped} skipped` : ""
          }. They apply on the agent's next report.`,
        });
      },
      onError: (failure) => toast.add({ title: (failure as Error).message || "Unable to accept" }),
    },
  });

  // High-confidence suggestions are ticked by default.
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
  const chosen = active.filter((item) => canAccept(item) && selected.has(item.id ?? ""));
  const toggle = (id: string) =>
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  const act = (id: string | undefined, type: "dismiss" | "restore" | "ignore") =>
    requests.send(type, { ids: [id] });

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

  const problems = (discovery.kinds ?? []).filter(
    (kind) => kind.state && !["ok", "off", "disabled"].includes(kind.state),
  );
  const settings = (
    <DetectedSettings
      enabled={discovery.enabled !== false}
      kinds={discovery.kinds ?? []}
      canAct={canAct}
      onScanning={(enabled) => requests.send("set_scanning", { enabled })}
      onKind={(kind, enabled) => requests.send("set_kind", { kind, enabled })}
    />
  );

  return (
    <div className="flex flex-col gap-3.5">
      <div className="flex flex-wrap items-center justify-between gap-2 text-[13px] text-quiet">
        <span>
          <b className="font-medium text-ink">{discovery.things ?? 0} things found</b> ·{" "}
          {discovery.suggested ?? 0} suggested · {discovery.monitored ?? 0} monitored
          {(discovery.new_items ?? 0) > 0 && ` · ${discovery.new_items} new`}
        </span>
        <span className="flex items-center gap-3">
          Last scan {formatDate(discovery.scanned_at, "MMM d, HH:mm", "unknown")}
          <Button
            size="sm"
            variant="secondary"
            disabled={!canAct || requests.pending || discovery.enabled === false}
            onClick={() => requests.send("rescan")}
          >
            Rescan now
          </Button>
        </span>
      </div>
      {problems.map((kind) => (
        <Callout key={kind.kind} tone="warn" lead={`Could not look at ${kindTitle(kind.kind)}.`}>
          {kind.message}
          {kind.why ? ` ${kind.why}` : ""}
          {kind.fix && <CodeBlock className="mt-2">{kind.fix}</CodeBlock>}
        </Callout>
      ))}
      {discovery.enabled === false ? (
        <EmptyState
          className="rounded-xl border-solid bg-card"
          title="Scanning is off"
          description="Host metrics keep working. Turn scanning on below, or create monitors by hand."
          action={
            <Link to="/monitors/new/agent">
              <Button variant="secondary">Create monitor</Button>
            </Link>
          }
        />
      ) : active.length === 0 ? (
        <EmptyState
          className="rounded-xl border-solid bg-card"
          title="Nothing else detected"
          description="Host metrics are on. Rescan to look again."
        />
      ) : (
        <>
          {groupItems(active).map(([kind, group]) => (
            <FieldPanel key={kind} title={kindTitle(kind)} aside={`${group.length}`}>
              <RaisedCard>
                {group.map((item) => (
                  <DetectedItem
                    key={item.id}
                    item={item}
                    selectable={canAccept(item)}
                    checked={selected.has(item.id ?? "")}
                    canAct={canAct}
                    onToggle={() => toggle(item.id ?? "")}
                    onAction={(type) => act(item.id, type)}
                  />
                ))}
              </RaisedCard>
            </FieldPanel>
          ))}
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p className="max-w-[520px] text-[13px] text-quiet">
              Applies on the agent&apos;s next report, no restart. Log files need an ingest key, set
              up with <code>orion-agent setup</code>.
            </p>
            <Button
              disabled={chosen.length === 0 || accept.isPending}
              onClick={() =>
                accept.mutate({ agentId, data: { item_ids: chosen.map((item) => item.id ?? "") } })
              }
            >
              {accept.isPending ? "Adding..." : `Accept selected (${chosen.length})`}
            </Button>
          </div>
        </>
      )}
      {dismissed.length > 0 && (
        <FieldPanel title="Dismissed" aside={`${dismissed.length}`}>
          <RaisedCard>
            {dismissed.map((item) => (
              <DetectedItem
                key={item.id}
                item={item}
                selectable={false}
                checked={false}
                canAct={canAct}
                onToggle={() => undefined}
                onAction={(type) => act(item.id, type)}
              />
            ))}
          </RaisedCard>
        </FieldPanel>
      )}
      <DetectedRequests agentId={agentId} onCancel={requests.cancel} />
      {settings}
    </div>
  );
};
