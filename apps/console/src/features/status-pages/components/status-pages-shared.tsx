import { StatusBadge } from "@/components/shared/status-badges";
import { Input } from "@/components/ui/input";
import {
  type ApiIncidentResponse,
  type ApiStatusPageIncidentComponentSuggestionResponse,
  type ApiStatusPageIncidentResponse,
  type ApiStatusPageResponse,
} from "@/orion-sdk";
import { type Dispatch, type ReactNode, type SetStateAction } from "react";

export type PageFormState = {
  slug: string;
  title: string;
  description: string;
};

export type PageSettingsFormState = {
  description: string;
  seoTitle: string;
  seoDescription: string;
  canonicalUrl: string;
  openGraphImageUrl: string;
  accentColor: string;
  logoUrl: string;
  logoAlt: string;
  headerStyle: string;
  componentDensity: string;
  themeMode: string;
  showUptimeSummary: boolean;
  showIncidentHistory: boolean;
  defaultIncidentVisibility: string;
};

export type SectionFormState = {
  name: string;
};

export type ComponentFormState = {
  sectionId: string;
  publicName: string;
  publicDescription: string;
  manualStatus: string;
};

export type MappingFormState = {
  componentId: string;
  resourceType: "monitor" | "agent";
  resourceId: string;
};

export type IncidentFormState = {
  internalIncidentId: string;
  title: string;
  publicStatus: string;
  severity: string;
  impactSummary: string;
  visibility: string;
  affectedComponentIds: string[];
  publishedAt: string;
  resolvedAt: string;
  scheduledStartAt: string;
  scheduledEndAt: string;
};

export type IncidentUpdateFormState = {
  status: string;
  message: string;
  createdBy: string;
  publishedAt: string;
};

export type StateSetter<T> = Dispatch<SetStateAction<T>>;

export const emptyPageForm: PageFormState = {
  slug: "",
  title: "",
  description: "",
};

export const emptyPageSettingsForm: PageSettingsFormState = {
  description: "",
  seoTitle: "",
  seoDescription: "",
  canonicalUrl: "",
  openGraphImageUrl: "",
  accentColor: "#0f766e",
  logoUrl: "",
  logoAlt: "",
  headerStyle: "standard",
  componentDensity: "comfortable",
  themeMode: "light",
  showUptimeSummary: true,
  showIncidentHistory: true,
  defaultIncidentVisibility: "draft",
};

export const emptySectionForm: SectionFormState = { name: "" };

export const emptyComponentForm: ComponentFormState = {
  sectionId: "",
  publicName: "",
  publicDescription: "",
  manualStatus: "",
};

export const emptyMappingForm: MappingFormState = {
  componentId: "",
  resourceType: "monitor",
  resourceId: "",
};

export const emptyIncidentForm: IncidentFormState = {
  internalIncidentId: "",
  title: "",
  publicStatus: "investigating",
  severity: "medium",
  impactSummary: "",
  visibility: "draft",
  affectedComponentIds: [],
  publishedAt: "",
  resolvedAt: "",
  scheduledStartAt: "",
  scheduledEndAt: "",
};

export const emptyIncidentUpdateForm: IncidentUpdateFormState = {
  status: "investigating",
  message: "",
  createdBy: "",
  publishedAt: "",
};

export const manualStatuses = [
  { label: "No override", value: "" },
  { label: "Operational", value: "operational" },
  { label: "Degraded", value: "degraded" },
  { label: "Partial outage", value: "partial_outage" },
  { label: "Major outage", value: "major_outage" },
  { label: "Maintenance", value: "maintenance" },
  { label: "Unknown", value: "unknown" },
];

export const incidentStatuses = [
  { label: "Investigating", value: "investigating" },
  { label: "Identified", value: "identified" },
  { label: "Monitoring", value: "monitoring" },
  { label: "Resolved", value: "resolved" },
  { label: "Scheduled", value: "scheduled" },
];

export const incidentSeverities = [
  { label: "Low", value: "low" },
  { label: "Medium", value: "medium" },
  { label: "High", value: "high" },
  { label: "Critical", value: "critical" },
];

export const incidentVisibilities = [
  { label: "Draft", value: "draft" },
  { label: "Published", value: "published" },
  { label: "Private", value: "private" },
];

export const headerStyleOptions = [
  { label: "Standard", value: "standard" },
  { label: "Compact", value: "compact" },
  { label: "Centered", value: "centered" },
];

export const componentDensityOptions = [
  { label: "Comfortable", value: "comfortable" },
  { label: "Compact", value: "compact" },
];

export const themeModeOptions = [
  { label: "Light", value: "light" },
  { label: "Dark", value: "dark" },
  { label: "System", value: "system" },
];

export const subscriberStateOptions = [
  { label: "All subscribers", value: "" },
  { label: "Pending", value: "pending" },
  { label: "Confirmed", value: "confirmed" },
  { label: "Unsubscribed", value: "unsubscribed" },
  { label: "Bounced", value: "bounced" },
  { label: "Disabled", value: "disabled" },
];

export const statusBadgeStatus = (status?: string) => {
  switch (status) {
    case "operational":
      return "up";
    case "major_outage":
    case "partial_outage":
      return "down";
    case "maintenance":
      return "maintenance";
    case "degraded":
      return "degraded";
    default:
      return "unknown";
  }
};

export const subscriberBadgeStatus = (state?: string) => {
  switch (state) {
    case "confirmed":
      return "up";
    case "pending":
      return "maintenance";
    case "bounced":
      return "degraded";
    case "unsubscribed":
    case "disabled":
      return "stale";
    default:
      return "unknown";
  }
};

export const incidentBadgeStatus = (status?: string) => {
  switch (status) {
    case "resolved":
      return "up";
    case "scheduled":
    case "monitoring":
      return "maintenance";
    case "identified":
      return "degraded";
    case "investigating":
      return "down";
    default:
      return "unknown";
  }
};

export const publicUrl = (slug?: string) => (slug ? `/status/${slug}` : "");

export const dateTimeLocalToIso = (value: string) => {
  if (!value.trim()) return undefined;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return undefined;
  return date.toISOString();
};

export const isoToDateTimeLocal = (value?: string) => {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const localDate = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return localDate.toISOString().slice(0, 16);
};

export const formatDateTime = (value?: string) => {
  if (!value) return "Not set";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
};

const themeString = (settings: Record<string, unknown> | undefined, key: string, fallback = "") => {
  const value = settings?.[key];
  return typeof value === "string" ? value : fallback;
};

const themeBoolean = (
  settings: Record<string, unknown> | undefined,
  key: string,
  fallback: boolean,
) => {
  const value = settings?.[key];
  return typeof value === "boolean" ? value : fallback;
};

const validAccentColor = (value: string) =>
  /^#[0-9a-f]{6}$/i.test(value) ? value : emptyPageSettingsForm.accentColor;

export const pageSettingsFormFromPage = (page?: ApiStatusPageResponse): PageSettingsFormState => {
  const themeSettings = page?.theme_settings;
  return {
    accentColor: validAccentColor(
      themeString(themeSettings, "accent_color", emptyPageSettingsForm.accentColor),
    ),
    canonicalUrl: page?.canonical_url ?? "",
    componentDensity: themeString(
      themeSettings,
      "component_density",
      emptyPageSettingsForm.componentDensity,
    ),
    defaultIncidentVisibility:
      page?.default_incident_visibility ?? emptyPageSettingsForm.defaultIncidentVisibility,
    description: page?.description ?? "",
    headerStyle: themeString(themeSettings, "header_style", emptyPageSettingsForm.headerStyle),
    logoAlt: themeString(themeSettings, "logo_alt"),
    logoUrl: themeString(themeSettings, "logo_url"),
    openGraphImageUrl: page?.open_graph_image_url ?? "",
    seoDescription: page?.seo_description ?? "",
    seoTitle: page?.seo_title ?? "",
    themeMode: themeString(themeSettings, "theme_mode", emptyPageSettingsForm.themeMode),
    showIncidentHistory: themeBoolean(themeSettings, "show_incident_history", true),
    showUptimeSummary: themeBoolean(themeSettings, "show_uptime_summary", true),
  };
};

export const pageThemeSettings = (
  currentSettings: Record<string, unknown> | undefined,
  form: PageSettingsFormState,
) => ({
  ...currentSettings,
  accent_color: form.accentColor,
  component_density: form.componentDensity,
  header_style: form.headerStyle,
  logo_alt: form.logoAlt.trim() || undefined,
  logo_url: form.logoUrl.trim() || undefined,
  show_incident_history: form.showIncidentHistory,
  show_uptime_summary: form.showUptimeSummary,
  theme_mode: form.themeMode,
});

export const incidentFormFromIncident = (
  incident?: ApiStatusPageIncidentResponse,
): IncidentFormState => ({
  internalIncidentId: incident?.internal_incident_id ?? "",
  title: incident?.title ?? "",
  publicStatus: incident?.public_status ?? "investigating",
  severity: incident?.severity ?? "medium",
  impactSummary: incident?.impact_summary ?? "",
  visibility: incident?.visibility ?? "draft",
  affectedComponentIds: incident?.affected_component_ids ?? [],
  publishedAt: isoToDateTimeLocal(incident?.published_at),
  resolvedAt: isoToDateTimeLocal(incident?.resolved_at),
  scheduledStartAt: isoToDateTimeLocal(incident?.scheduled_start_at),
  scheduledEndAt: isoToDateTimeLocal(incident?.scheduled_end_at),
});

export const incidentOptionLabel = (incident: ApiIncidentResponse) =>
  `${incident.title ?? incident.id ?? "Untitled incident"}${incident.status ? ` (${incident.status})` : ""}`;

export const suggestionMatchLabel = (
  suggestion: ApiStatusPageIncidentComponentSuggestionResponse,
) =>
  (suggestion.matches ?? [])
    .map((match) => match.resource_type)
    .filter(Boolean)
    .join(", ");

export const toggleIncidentComponent = (
  form: IncidentFormState,
  setForm: StateSetter<IncidentFormState>,
  componentId: string,
) => {
  const hasComponent = form.affectedComponentIds.includes(componentId);
  setForm({
    ...form,
    affectedComponentIds: hasComponent
      ? form.affectedComponentIds.filter((id) => id !== componentId)
      : [...form.affectedComponentIds, componentId],
  });
};

export const applySuggestedIncidentComponents = (
  form: IncidentFormState,
  setForm: StateSetter<IncidentFormState>,
  suggestions: ApiStatusPageIncidentComponentSuggestionResponse[],
) => {
  const suggestedComponentIds = suggestions
    .map((suggestion) => suggestion.component_id)
    .filter((componentId): componentId is string => Boolean(componentId));
  if (suggestedComponentIds.length === 0) return;

  setForm({
    ...form,
    affectedComponentIds: Array.from(
      new Set([...form.affectedComponentIds, ...suggestedComponentIds]),
    ),
  });
};

export const incidentRequest = (form: IncidentFormState) => ({
  affected_component_ids: form.affectedComponentIds,
  impact_summary: form.impactSummary.trim() || undefined,
  internal_incident_id: form.internalIncidentId.trim() || undefined,
  public_status: form.publicStatus,
  published_at: dateTimeLocalToIso(form.publishedAt),
  resolved_at: dateTimeLocalToIso(form.resolvedAt),
  scheduled_end_at: dateTimeLocalToIso(form.scheduledEndAt),
  scheduled_start_at: dateTimeLocalToIso(form.scheduledStartAt),
  severity: form.severity,
  title: form.title.trim(),
  visibility: form.visibility,
});

export const Field = ({ label, children }: { label: string; children: ReactNode }) => (
  <label className="block space-y-1">
    <span className="text-sm font-medium">{label}</span>
    {children}
  </label>
);

export const IncidentStatusBadge = ({
  status,
  fallback,
}: {
  status?: string;
  fallback?: string;
}) => <StatusBadge fallback={fallback ?? status} value={incidentBadgeStatus(status)} />;

export const DateTimeInput = ({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) => (
  <Input type="datetime-local" value={value} onChange={(event) => onChange(event.target.value)} />
);
