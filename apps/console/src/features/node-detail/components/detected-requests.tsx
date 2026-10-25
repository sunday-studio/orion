import { MoreHorizontal } from "lucide-react";
import {
  FieldPanel,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
  RaisedCard,
  StatusBadge,
} from "@/design-system";
import { type ServiceNodeDiscoveryRequestView, useListNodeDiscoveryRequests } from "@/orion-sdk";
import { formatDate } from "@/utils/date";

const TITLES: Record<string, string> = {
  rescan: "Rescan",
  dismiss: "Dismiss",
  restore: "Restore",
  ignore: "Never suggest",
  unignore: "Suggest again",
  set_scanning: "Scanning",
  set_kind: "What to look at",
  track_storage: "Track storage",
  create_log_source: "Log source",
};

const minutesSince = (iso?: string) =>
  iso ? Math.max(Math.round((Date.now() - new Date(iso).getTime()) / 60000), 0) : 0;

/** Badge and words for a request, as in the spec sheet. */
const describe = (request: ServiceNodeDiscoveryRequestView) => {
  const waited = minutesSince(request.created_at);
  switch (request.status) {
    case "applied":
      return { tone: "up" as const, badge: "Done", words: request.result || "Done" };
    case "failed":
      return {
        tone: "down" as const,
        badge: "Failed",
        words: request.result || "The Node could not do this",
      };
    case "expired":
      return { tone: "stale" as const, badge: "Expired", words: "Nothing changed on the server" };
    case "unsupported":
      return {
        tone: "unknown" as const,
        badge: "Not supported",
        words: "This node cannot take requests from the Console. Update the node.",
      };
    case "cancelled":
      return { tone: "stale" as const, badge: "Cancelled", words: "" };
    default:
      return waited >= 5
        ? {
            tone: "degraded" as const,
            badge: "No answer yet",
            words: `Requested ${waited} minutes ago`,
          }
        : {
            tone: "degraded" as const,
            badge: "Waiting for the Node",
            words: `Requested at ${formatDate(request.created_at, "HH:mm")}`,
          };
  }
};

/** Waiting actions: Console requests queued for the node, with their outcome. */
export const DetectedRequests = ({
  nodeId,
  onCancel,
  onRetry,
}: {
  nodeId: string;
  onCancel: (requestId: string) => void;
  onRetry: (request: ServiceNodeDiscoveryRequestView) => void;
}) => {
  const query = useListNodeDiscoveryRequests(nodeId, {
    query: {
      refetchInterval: (state) =>
        (state.state.data?.requests ?? []).some(
          (r) => r.status === "queued" || r.status === "delivered",
        )
          ? 8000
          : false,
    },
  });
  const requests = (query.data?.requests ?? []).slice(0, 6);
  if (requests.length === 0) return null;
  return (
    <FieldPanel title="Waiting actions">
      <RaisedCard>
        {requests.map((request) => {
          const info = describe(request);
          return (
            <div
              key={request.id}
              className="grid min-h-[46px] grid-cols-[150px_190px_minmax(0,1fr)_110px] items-center gap-3 border-t border-line-soft px-3.5 py-1.5 text-[13px] first:border-t-0"
            >
              <span className="text-[12.5px] text-quiet">
                {TITLES[request.type ?? ""] ?? request.type}
              </span>
              <span>
                <StatusBadge status={info.tone}>{info.badge}</StatusBadge>
              </span>
              <span className="truncate">{info.words}</span>
              <span className="flex justify-end">
                <Menu>
                  <MenuTrigger
                    aria-label="Actions"
                    className="inline-flex h-7 items-center gap-1.5 rounded-md border border-input-border bg-card px-2.5 text-[13px] outline-none hover:bg-sub focus-visible:ring-2 focus-visible:ring-focus"
                  >
                    Actions <MoreHorizontal className="size-3.5 opacity-60" />
                  </MenuTrigger>
                  <MenuContent>
                    <MenuItem onClick={() => onRetry(request)}>Try again</MenuItem>
                    {request.status === "queued" && (
                      <MenuItem onClick={() => onCancel(request.id ?? "")}>Cancel request</MenuItem>
                    )}
                  </MenuContent>
                </Menu>
              </span>
            </div>
          );
        })}
      </RaisedCard>
    </FieldPanel>
  );
};
