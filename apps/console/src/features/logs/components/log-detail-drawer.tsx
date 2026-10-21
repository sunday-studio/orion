import { Link } from "react-router-dom";
import {
  Button,
  Callout,
  CodeBlock,
  Drawer,
  DrawerContent,
  DrawerTitle,
  JsonBlock,
  RaisedCard,
  Skeleton,
  useToast,
} from "@/design-system";
import { useGetLogEntry, useGetLogRelated } from "@/orion-sdk";
import { formatDate } from "@/utils/date";
import { type LogRow, sourceColor } from "../log-model";
import type { LogKind, LogQueryState } from "../logs-state";
import { LevelBadge } from "./log-states";

const Meta = ({
  label,
  action,
  children,
}: {
  label: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) => (
  <div className="grid min-h-[43px] grid-cols-[110px_minmax(0,1fr)_auto] items-center gap-3 border-t border-line-soft px-3.5 text-[13px] first:border-t-0">
    <span className="text-quiet">{label}</span>
    <span className="flex min-w-0 items-center gap-2 truncate">{children}</span>
    {action}
  </div>
);

const small = "h-7 rounded-md px-2 text-xs";

const ServiceDetail = ({
  id,
  kind,
  onFilter,
}: {
  id: string;
  kind: LogKind;
  onFilter: (patch: Partial<LogQueryState>) => void;
}) => {
  const toast = useToast();
  const entry = useGetLogEntry(id);
  const related = useGetLogRelated(id, { limit: 8 });
  const data = entry.data?.entry;
  if (entry.isPending) return <Skeleton className="h-64 w-full" />;
  if (!data) return <Callout tone="down">This log line is no longer available.</Callout>;
  const copy = async (text: string, name: string) => {
    await navigator.clipboard.writeText(text);
    toast.add({ title: `${name} copied` });
  };
  const hasAttributes = Object.keys(data.attributes ?? {}).length > 0;
  const plain = !data.event && !hasAttributes;
  const relatedBy = related.data?.by;
  return (
    <div className="grid gap-4">
      <RaisedCard>
        <Meta
          label="Time"
          action={
            <Button
              size="sm"
              variant="secondary"
              className={small}
              onClick={() => void copy(data.timestamp ?? "", "Time")}
            >
              Copy
            </Button>
          }
        >
          {formatDate(data.timestamp, "MMM d, HH:mm:ss.SSS")} UTC
        </Meta>
        <Meta
          label={kind === "internal" ? "Component" : "Source"}
          action={
            kind === "service" && data.app_id ? (
              <Button
                size="sm"
                variant="secondary"
                className={small}
                onClick={() => onFilter({ source: data.app_id ?? "", log: "" })}
              >
                Filter
              </Button>
            ) : undefined
          }
        >
          <i
            className="size-2 shrink-0 rounded-[2px]"
            style={{ background: sourceColor(data.app_name) }}
          />
          {data.app_name ??
            (data.attributes as { component?: string } | undefined)?.component ??
            "node"}
        </Meta>
        {data.request_id && (
          <Meta
            label="Request"
            action={
              <Button
                size="sm"
                variant="secondary"
                className={small}
                onClick={() => onFilter({ q: data.request_id ?? "", log: "" })}
              >
                Filter
              </Button>
            }
          >
            <code className="font-mono text-[12.5px]">{data.request_id}</code>
          </Meta>
        )}
        {data.trace_id && (
          <Meta
            label="Trace"
            action={
              <Button
                size="sm"
                variant="secondary"
                className={small}
                onClick={() => onFilter({ q: data.trace_id ?? "", log: "" })}
              >
                Open trace
              </Button>
            }
          >
            <code className="font-mono text-[12.5px]">{data.trace_id}</code>
          </Meta>
        )}
      </RaisedCard>
      <section className="grid gap-1.5">
        <h3 className="text-[13px] font-medium">Message</h3>
        {plain ? (
          <CodeBlock className="whitespace-pre-wrap break-words">{data.body}</CodeBlock>
        ) : (
          <p className="text-[13px] leading-[1.5]">{data.body}</p>
        )}
      </section>
      {plain ? (
        <Callout tone="warn">
          No attributes. This app sends plain text, so there are no fields to filter on.
        </Callout>
      ) : (
        hasAttributes && (
          <section className="grid gap-1.5">
            <h3 className="text-[13px] font-medium">Attributes</h3>
            <JsonBlock value={data.attributes} />
          </section>
        )
      )}
      <div className="flex gap-2">
        <Button
          size="sm"
          variant="secondary"
          onClick={() =>
            void copy(
              plain ? (data.body ?? "") : JSON.stringify(data, null, 2),
              plain ? "Line" : "JSON",
            )
          }
        >
          {plain ? "Copy line" : "Copy as JSON"}
        </Button>
        <Button
          size="sm"
          variant="secondary"
          onClick={() => void copy(window.location.href, "Link")}
        >
          Copy link
        </Button>
      </div>
      {(related.data?.entries?.length ?? 0) > 0 && (
        <section className="grid gap-1.5">
          <h3 className="text-[13px] font-medium">
            {relatedBy === "nearby" ? "Nearby lines" : "Related logs"}
          </h3>
          <RaisedCard>
            {(related.data?.entries ?? []).map((item) => (
              <div
                key={item.id}
                className="grid grid-cols-[96px_80px_minmax(0,1fr)] items-center gap-2.5 border-t border-line-soft px-3.5 py-2.5 text-[13px] first:border-t-0"
              >
                <span className="tabular-nums text-faint">
                  {formatDate(item.timestamp, "HH:mm:ss.SSS")}
                </span>
                <LevelBadge level={item.severity} />
                <b className={`truncate ${item.id === id ? "font-medium" : "font-normal"}`}>
                  {item.event ?? item.body}
                </b>
              </div>
            ))}
          </RaisedCard>
          {relatedBy !== "nearby" &&
            (related.data?.total ?? 0) > (related.data?.entries?.length ?? 0) && (
              <button
                type="button"
                className="w-fit text-[13px] underline"
                onClick={() =>
                  onFilter({
                    q: (relatedBy === "trace_id" ? data.trace_id : data.request_id) ?? "",
                    log: "",
                  })
                }
              >
                Show all {related.data?.total} logs for {related.data?.value}
              </button>
            )}
        </section>
      )}
    </div>
  );
};

const StatusDetail = ({ row }: { row?: LogRow }) =>
  row ? (
    <div className="grid gap-4">
      <RaisedCard>
        <Meta label="Time">{formatDate(row.at, "MMM d, HH:mm:ss")} UTC</Meta>
        {row.label && <Meta label="Server">{row.label}</Meta>}
        {row.monitor && <Meta label="Monitor">{row.monitor}</Meta>}
      </RaisedCard>
      <section className="grid gap-1.5">
        <h3 className="text-[13px] font-medium">Event</h3>
        <p className="text-[13px] leading-[1.5]">
          <b className="font-medium">{row.title}</b> {row.detail}
        </p>
      </section>
      <Link to="/incidents" className="w-fit text-[13px] underline">
        Open incidents
      </Link>
    </div>
  ) : (
    <Callout tone="warn">This event is outside the loaded page.</Callout>
  );

export const LogDetailDrawer = ({
  id,
  kind,
  statusRow,
  onClose,
  onFilter,
}: {
  id: string;
  kind: LogKind;
  statusRow?: LogRow;
  onClose: () => void;
  onFilter: (patch: Partial<LogQueryState>) => void;
}) => (
  <Drawer open={Boolean(id)} onOpenChange={(open) => !open && onClose()} swipeDirection="right">
    <DrawerContent className="w-[480px] gap-4 p-5">
      <div className="flex items-center gap-2 pr-9">
        <DrawerTitle className="pr-0 text-[15px]">
          {kind === "status" ? (statusRow?.title ?? "Event") : "Log line"}
        </DrawerTitle>
        {kind === "status" && statusRow && <LevelBadge level={statusRow.level} />}
      </div>
      {id &&
        (kind === "status" ? (
          <StatusDetail row={statusRow} />
        ) : (
          <ServiceDetail key={id} id={id} kind={kind} onFilter={onFilter} />
        ))}
    </DrawerContent>
  </Drawer>
);
