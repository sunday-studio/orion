import { useState } from "react";
import {
  Button,
  EmptyState,
  FieldPanel,
  RaisedCard,
  Segmented,
  StatusBadge,
  Textarea,
  useToast,
} from "@/design-system";
import { type ApiStatusPageDetailResponse, useCreateStatusPageIncidentUpdate } from "@/orion-sdk";
import { formatDate } from "@/utils/date";

const STEPS = [
  { value: "investigating", label: "Investigating" },
  { value: "identified", label: "Identified" },
  { value: "monitoring", label: "Monitoring" },
  { value: "resolved", label: "Resolved" },
];

export const IncidentsTab = ({
  pageId,
  detail,
  onChanged,
}: {
  pageId: string;
  detail?: ApiStatusPageDetailResponse;
  onChanged: () => void;
}) => {
  const toast = useToast();
  const incidents = detail?.incidents ?? [];
  const [selected, setSelected] = useState<string>();
  const [status, setStatus] = useState("investigating");
  const [message, setMessage] = useState("");
  const current = incidents.find((i) => i.id === selected) ?? incidents[0];
  const post = useCreateStatusPageIncidentUpdate({
    mutation: {
      onSuccess: () => {
        setMessage("");
        onChanged();
        toast.add({ title: "Update posted" });
      },
      onError: () => toast.add({ title: "Unable to post the update" }),
    },
  });

  if (incidents.length === 0) {
    return (
      <EmptyState
        title="No public incidents"
        description="Incidents you publish to this page appear here."
      />
    );
  }
  return (
    <div className="flex flex-col gap-4">
      <FieldPanel title="Public incidents" aside={`${incidents.length}`}>
        <RaisedCard>
          {incidents.map((incident) => (
            <div
              key={incident.id}
              className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3 border-t border-line-soft px-3.5 py-2.5 text-[13px] first:border-t-0"
            >
              <span className="min-w-0">
                <b className="block truncate font-medium">{incident.title}</b>
                <small className="text-xs text-faint">
                  {formatDate(incident.published_at ?? incident.created_at, "MMM d, HH:mm")}
                </small>
              </span>
              <StatusBadge status={incident.resolved_at ? "up" : "degraded"}>
                {incident.public_status ?? "investigating"}
              </StatusBadge>
              <Button
                size="sm"
                variant="secondary"
                disabled={incident.id === current?.id}
                onClick={() => {
                  setSelected(incident.id);
                  setStatus(incident.public_status ?? "investigating");
                  setMessage("");
                }}
              >
                {incident.id === current?.id ? "Selected" : "Write update"}
              </Button>
            </div>
          ))}
        </RaisedCard>
      </FieldPanel>
      {current && (
        <FieldPanel title="Post an update" aside={current.title}>
          <RaisedCard className="grid gap-3 p-4">
            <Segmented
              aria-label="Update status"
              surface="card"
              options={STEPS}
              value={status}
              onValueChange={setStatus}
            />
            <Textarea
              rows={3}
              placeholder="What changed?"
              value={message}
              onChange={(event) => setMessage(event.target.value)}
            />
            <div className="flex justify-end">
              <Button
                disabled={post.isPending || !message.trim()}
                onClick={() =>
                  post.mutate({
                    id: pageId,
                    incidentId: current.id ?? "",
                    data: { status, message: message.trim() },
                  })
                }
              >
                Post update
              </Button>
            </div>
          </RaisedCard>
        </FieldPanel>
      )}
    </div>
  );
};
