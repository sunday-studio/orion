import { Button, FieldPanel, RaisedCard, StatusBadge } from "@/design-system";
import { type ServiceAgentDiscoveryRequestView, useListAgentDiscoveryRequests } from "@/orion-sdk";
import { formatDate } from "@/utils/date";

const TITLES: Record<string, string> = {
  rescan: "Rescan",
  dismiss: "Dismiss",
  restore: "Restore",
  ignore: "Never suggest",
  unignore: "Suggest again",
  set_scanning: "Scanning on or off",
  set_kind: "Kind on or off",
  track_storage: "Track storage",
  create_log_source: "Log source",
};

const tone = (status?: string) =>
  status === "applied"
    ? "up"
    : status === "failed"
      ? "down"
      : status === "queued" || status === "delivered"
        ? "degraded"
        : "stale";

const note = (request: ServiceAgentDiscoveryRequestView) => {
  if (request.result) return request.result;
  if (request.status === "queued") return "Waiting for the agent's next check-in";
  if (request.status === "delivered") return "The agent has it and is working on it";
  if (request.status === "unsupported")
    return "This agent cannot take requests from the Console yet";
  if (request.status === "expired") return "The agent did not pick it up in time";
  return "";
};

/** Console actions that are queued for the agent, with their outcome in the agent's own words. */
export const DetectedRequests = ({
  agentId,
  onCancel,
}: {
  agentId: string;
  onCancel: (requestId: string) => void;
}) => {
  const query = useListAgentDiscoveryRequests(agentId, {
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
    <FieldPanel title="Requests" aside="Applied by the agent at its next check-in">
      <RaisedCard>
        {requests.map((request) => (
          <div
            key={request.id}
            className="grid min-h-[42px] grid-cols-[150px_minmax(0,1fr)_auto_auto] items-center gap-3 border-t border-line-soft px-3.5 py-2 text-[13px] first:border-t-0"
          >
            <b className="font-medium">{TITLES[request.type ?? ""] ?? request.type}</b>
            <span className="truncate text-quiet">{note(request)}</span>
            <span className="text-xs text-faint">{formatDate(request.created_at, "HH:mm")}</span>
            <span className="flex items-center gap-2">
              <StatusBadge status={tone(request.status)}>{request.status}</StatusBadge>
              {request.status === "queued" && (
                <Button size="sm" variant="secondary" onClick={() => onCancel(request.id ?? "")}>
                  Cancel
                </Button>
              )}
            </span>
          </div>
        ))}
      </RaisedCard>
    </FieldPanel>
  );
};
