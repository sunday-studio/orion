import { EmptyState } from "@/components/shared/empty-state";
import { StatusBadge } from "@/components/shared/status-badges";
import { Button } from "@/components/ui/button";
import type {
  ApiStatusPagePreviewResponse,
  ApiStatusPagePublicComponentResponse,
} from "@/orion-sdk";
import { CheckCircle2, Eye } from "lucide-react";
import { formatDateTime, incidentBadgeStatus, statusBadgeStatus } from "./status-pages-shared";

const uptimeStatusClass = (status?: string) => {
  switch (status) {
    case "operational":
      return "bg-emerald-500";
    case "degraded":
      return "bg-amber-400";
    case "partial_outage":
    case "major_outage":
    case "outage":
      return "bg-red-500";
    case "maintenance":
      return "bg-blue-500";
    case "no_data":
    case "unknown":
      return "bg-neutral-300";
    default:
      return "bg-neutral-300";
  }
};

const previewStatusPanelClass = (status?: string) => {
  switch (status) {
    case "operational":
      return "border-emerald-300 bg-emerald-50 text-emerald-950";
    case "degraded":
      return "border-amber-300 bg-amber-50 text-amber-950";
    case "partial_outage":
    case "major_outage":
      return "border-red-300 bg-red-50 text-red-950";
    case "maintenance":
      return "border-blue-300 bg-blue-50 text-blue-950";
    default:
      return "border-neutral-200 bg-neutral-50 text-neutral-900";
  }
};

const previewStatusMessage = (status?: string) => {
  switch (status) {
    case "operational":
      return "All systems operational";
    case "degraded":
      return "Some systems degraded";
    case "partial_outage":
      return "Partial outage";
    case "major_outage":
      return "Major outage";
    case "maintenance":
      return "Maintenance in progress";
    default:
      return "Status unavailable";
  }
};

type StatusPagePreviewPanelProps = {
  isError: boolean;
  isLoading: boolean;
  preview?: ApiStatusPagePreviewResponse;
  previewDark: boolean;
};

export const StatusPagePreviewPanel = ({
  isError,
  isLoading,
  preview,
  previewDark,
}: StatusPagePreviewPanelProps) => {
  const activeIncidents = (preview?.incidents ?? []).filter(
    (incident) => incident.public_status !== "resolved",
  );
  const recentIncidents = (preview?.incidents ?? []).filter(
    (incident) => incident.public_status === "resolved",
  );

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium">Preview</h3>
        <Eye className="size-4 text-neutral-500" />
      </div>
      {isLoading && <div className="text-sm text-neutral-600">Loading...</div>}
      {isError && (
        <EmptyState
          className="min-h-32"
          title="Unable to load public preview"
          description="Retry after Core is reachable."
          tone="error"
        />
      )}
      {preview && (
        <div
          className={`space-y-4 border p-4 ${
            previewDark
              ? "border-neutral-800 bg-neutral-950 text-neutral-100"
              : "border-neutral-200 bg-neutral-50 text-neutral-950"
          }`}
        >
          <PreviewHeader preview={preview} previewDark={previewDark} />
          <div className={`rounded border p-3 ${previewStatusPanelClass(preview.overall_status)}`}>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <CheckCircle2 className="size-5" />
                <span className="font-semibold">
                  {previewStatusMessage(preview.overall_status)}
                </span>
              </div>
              <span className="text-xs">Updated {formatDateTime(preview.last_updated)}</span>
            </div>
          </div>
          <PreviewActiveIncidents incidents={activeIncidents} previewDark={previewDark} />
          <PreviewSections preview={preview} previewDark={previewDark} />
          <PreviewRecentIncidents incidents={recentIncidents} previewDark={previewDark} />
        </div>
      )}
    </div>
  );
};

const PreviewHeader = ({
  preview,
  previewDark,
}: {
  preview: NonNullable<StatusPagePreviewPanelProps["preview"]>;
  previewDark: boolean;
}) => (
  <div className="flex flex-wrap items-center justify-between gap-3">
    <div>
      <div className="font-semibold">{preview.page?.title}</div>
      <div className={previewDark ? "text-sm text-neutral-400" : "text-sm text-neutral-600"}>
        {preview.page?.description || preview.page?.slug}
      </div>
    </div>
    <Button size="sm" type="button" variant="outline">
      Get updates
    </Button>
  </div>
);

const PreviewActiveIncidents = ({
  incidents,
  previewDark,
}: {
  incidents: NonNullable<StatusPagePreviewPanelProps["preview"]>["incidents"];
  previewDark: boolean;
}) =>
  incidents && incidents.length > 0 ? (
    <div className="space-y-2">
      <div className="text-sm font-medium">Active events</div>
      {incidents.map((incident) => (
        <div
          className={
            previewDark
              ? "border border-neutral-800 bg-neutral-900 p-3 text-sm"
              : "border border-neutral-200 bg-white p-3 text-sm"
          }
          key={incident.id}
        >
          <div className="flex items-center justify-between gap-2">
            <span className="font-medium">{incident.title}</span>
            <StatusBadge
              fallback={incident.public_status}
              value={incidentBadgeStatus(incident.public_status)}
            />
          </div>
          {incident.impact_summary && (
            <p className={previewDark ? "mt-1 text-neutral-400" : "mt-1 text-neutral-600"}>
              {incident.impact_summary}
            </p>
          )}
        </div>
      ))}
    </div>
  ) : null;

const PreviewSections = ({
  preview,
  previewDark,
}: {
  preview: NonNullable<StatusPagePreviewPanelProps["preview"]>;
  previewDark: boolean;
}) => (
  <div className="space-y-4">
    {(preview.sections ?? []).map((section) => (
      <div key={section.id}>
        <div className="mb-2 text-sm font-medium">{section.name}</div>
        <div className="space-y-3">
          {(section.components ?? []).map((component: ApiStatusPagePublicComponentResponse) => (
            <div className="space-y-2 text-sm" key={component.id}>
              <div className="flex items-center justify-between gap-3">
                <span className="font-medium">{component.name}</span>
                <span className="flex items-center gap-2">
                  <span className={previewDark ? "text-neutral-400" : "text-neutral-600"}>
                    {component.uptime?.uptime_display ?? "No data"}
                  </span>
                  <StatusBadge
                    fallback={component.status}
                    value={statusBadgeStatus(component.status)}
                  />
                </span>
              </div>
              {(component.uptime_history ?? []).length > 0 && (
                <div
                  aria-label={`${component.name} uptime history`}
                  className="grid gap-0.5"
                  style={{
                    gridTemplateColumns: `repeat(${component.uptime_history?.length ?? 1}, minmax(1px, 1fr))`,
                  }}
                >
                  {(component.uptime_history ?? []).map((bucket) => (
                    <span
                      aria-label={`${bucket.date}: ${bucket.uptime_display}`}
                      className={`h-6 rounded-sm ${uptimeStatusClass(bucket.status)}`}
                      key={bucket.date}
                      title={`${bucket.date}: ${bucket.uptime_display}`}
                    />
                  ))}
                </div>
              )}
              <div className="flex justify-between text-xs text-neutral-500">
                <span>{component.uptime_history?.[0]?.date ?? preview.uptime_window}</span>
                <span>today</span>
              </div>
            </div>
          ))}
        </div>
      </div>
    ))}
  </div>
);

const PreviewRecentIncidents = ({
  incidents,
  previewDark,
}: {
  incidents: NonNullable<StatusPagePreviewPanelProps["preview"]>["incidents"];
  previewDark: boolean;
}) => (
  <div className="space-y-2">
    <div className="text-sm font-medium">Recent events</div>
    {(!incidents || incidents.length === 0) && (
      <div className={previewDark ? "text-sm text-neutral-400" : "text-sm text-neutral-600"}>
        No recent incidents.
      </div>
    )}
    {(incidents ?? []).slice(0, 3).map((incident) => (
      <div
        className={
          previewDark
            ? "border-t border-neutral-800 pt-2 text-sm"
            : "border-t border-neutral-200 pt-2 text-sm"
        }
        key={incident.id}
      >
        <div className="flex items-center justify-between gap-2">
          <span className="font-medium">{incident.title}</span>
          <span className={previewDark ? "text-neutral-400" : "text-neutral-600"}>
            {incident.public_status}
          </span>
        </div>
      </div>
    ))}
  </div>
);
