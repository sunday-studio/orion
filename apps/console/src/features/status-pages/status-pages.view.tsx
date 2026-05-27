import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { StatusBadge } from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  type ApiIncidentResponse,
  type ApiStatusPageIncidentComponentSuggestionResponse,
  type ApiStatusPageIncidentResponse,
  type ApiStatusPagePublicComponentResponse,
  type ApiStatusPageResponse,
  useCreateStatusPage,
  useCreateStatusPageComponent,
  useCreateStatusPageComponentMapping,
  useCreateStatusPageIncident,
  useCreateStatusPageIncidentUpdate,
  useCreateStatusPageSection,
  useGetAgents,
  useGetIncidents,
  useGetMonitors,
  useGetStatusPage,
  useListStatusPages,
  usePreviewStatusPage,
  usePublishStatusPage,
  useSuggestStatusPageIncidentComponents,
  useUnpublishStatusPage,
  useUpdateStatusPage,
  useUpdateStatusPageIncident,
} from "@/orion-sdk";
import { CheckCircle2, ExternalLink, Eye, Globe2, Link2, Plus, RadioTower } from "lucide-react";
import { type FormEvent, type ReactNode, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";

type PageFormState = {
  slug: string;
  title: string;
  description: string;
};

type PageSettingsFormState = {
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
  showUptimeSummary: boolean;
  showIncidentHistory: boolean;
  defaultIncidentVisibility: string;
};

type SectionFormState = {
  name: string;
};

type ComponentFormState = {
  sectionId: string;
  publicName: string;
  publicDescription: string;
  manualStatus: string;
};

type MappingFormState = {
  componentId: string;
  resourceType: "monitor" | "agent";
  resourceId: string;
};

type IncidentFormState = {
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

type IncidentUpdateFormState = {
  status: string;
  message: string;
  createdBy: string;
  publishedAt: string;
};

const emptyPageForm: PageFormState = {
  slug: "",
  title: "",
  description: "",
};

const emptyPageSettingsForm: PageSettingsFormState = {
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
  showUptimeSummary: true,
  showIncidentHistory: true,
  defaultIncidentVisibility: "draft",
};

const emptySectionForm: SectionFormState = {
  name: "",
};

const emptyComponentForm: ComponentFormState = {
  sectionId: "",
  publicName: "",
  publicDescription: "",
  manualStatus: "",
};

const emptyMappingForm: MappingFormState = {
  componentId: "",
  resourceType: "monitor",
  resourceId: "",
};

const emptyIncidentForm: IncidentFormState = {
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

const emptyIncidentUpdateForm: IncidentUpdateFormState = {
  status: "investigating",
  message: "",
  createdBy: "",
  publishedAt: "",
};

const manualStatuses = [
  { label: "No override", value: "" },
  { label: "Operational", value: "operational" },
  { label: "Degraded", value: "degraded" },
  { label: "Partial outage", value: "partial_outage" },
  { label: "Major outage", value: "major_outage" },
  { label: "Maintenance", value: "maintenance" },
  { label: "Unknown", value: "unknown" },
];

const incidentStatuses = [
  { label: "Investigating", value: "investigating" },
  { label: "Identified", value: "identified" },
  { label: "Monitoring", value: "monitoring" },
  { label: "Resolved", value: "resolved" },
  { label: "Scheduled", value: "scheduled" },
];

const incidentSeverities = [
  { label: "Low", value: "low" },
  { label: "Medium", value: "medium" },
  { label: "High", value: "high" },
  { label: "Critical", value: "critical" },
];

const incidentVisibilities = [
  { label: "Draft", value: "draft" },
  { label: "Published", value: "published" },
  { label: "Private", value: "private" },
];

const headerStyleOptions = [
  { label: "Standard", value: "standard" },
  { label: "Compact", value: "compact" },
  { label: "Centered", value: "centered" },
];

const componentDensityOptions = [
  { label: "Comfortable", value: "comfortable" },
  { label: "Compact", value: "compact" },
];

const statusBadgeStatus = (status?: string) => {
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

const incidentBadgeStatus = (status?: string) => {
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

const publicUrl = (slug?: string) => (slug ? `/status/${slug}` : "");

const dateTimeLocalToIso = (value: string) => {
  if (!value.trim()) return undefined;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return undefined;
  return date.toISOString();
};

const isoToDateTimeLocal = (value?: string) => {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const localDate = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return localDate.toISOString().slice(0, 16);
};

const formatDateTime = (value?: string) => {
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

const pageSettingsFormFromPage = (page?: ApiStatusPageResponse): PageSettingsFormState => {
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
    showIncidentHistory: themeBoolean(themeSettings, "show_incident_history", true),
    showUptimeSummary: themeBoolean(themeSettings, "show_uptime_summary", true),
  };
};

const pageThemeSettings = (
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
});

const incidentFormFromIncident = (incident?: ApiStatusPageIncidentResponse): IncidentFormState => ({
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

const incidentOptionLabel = (incident: ApiIncidentResponse) =>
  `${incident.title ?? incident.id ?? "Untitled incident"}${incident.status ? ` (${incident.status})` : ""}`;

const suggestionMatchLabel = (suggestion: ApiStatusPageIncidentComponentSuggestionResponse) =>
  (suggestion.matches ?? [])
    .map((match) => match.resource_type)
    .filter(Boolean)
    .join(", ");

const Field = ({ label, children }: { label: string; children: ReactNode }) => (
  <label className="block space-y-1">
    <span className="text-sm font-medium">{label}</span>
    {children}
  </label>
);

export const StatusPagesPage = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedPageId = searchParams.get("page") ?? "";
  const pagesResponse = useListStatusPages();
  const pages = useMemo(() => pagesResponse.data?.pages ?? [], [pagesResponse.data]);
  const selectedPage = pages.find((page) => page.id === selectedPageId) ?? pages[0];
  const pageId = selectedPage?.id ?? "";
  const detailResponse = useGetStatusPage(pageId, { query: { enabled: Boolean(pageId) } });
  const previewResponse = usePreviewStatusPage(pageId, { query: { enabled: Boolean(pageId) } });
  const monitorsResponse = useGetMonitors({ limit: 200 });
  const agentsResponse = useGetAgents({ limit: 200 });
  const internalIncidentsResponse = useGetIncidents({
    limit: 50,
    offset: 0,
    status: "open,acknowledged,resolved",
  });
  const [pageForm, setPageForm] = useState<PageFormState>(emptyPageForm);
  const [pageSettingsForm, setPageSettingsForm] =
    useState<PageSettingsFormState>(emptyPageSettingsForm);
  const [sectionForm, setSectionForm] = useState<SectionFormState>(emptySectionForm);
  const [componentForm, setComponentForm] = useState<ComponentFormState>(emptyComponentForm);
  const [mappingForm, setMappingForm] = useState<MappingFormState>(emptyMappingForm);
  const [createIncidentForm, setCreateIncidentForm] =
    useState<IncidentFormState>(emptyIncidentForm);
  const [editIncidentForm, setEditIncidentForm] = useState<IncidentFormState>(emptyIncidentForm);
  const [updateForm, setUpdateForm] = useState<IncidentUpdateFormState>(emptyIncidentUpdateForm);
  const [selectedIncidentId, setSelectedIncidentId] = useState("");

  useEffect(() => {
    if (!selectedPageId && pages[0]?.id) {
      setSearchParams({ page: pages[0].id });
    }
  }, [pages, selectedPageId, setSearchParams]);

  useEffect(() => {
    const firstSection = detailResponse.data?.sections?.[0]?.id ?? "";
    setComponentForm((current) =>
      current.sectionId || !firstSection ? current : { ...current, sectionId: firstSection },
    );
    const firstComponent = detailResponse.data?.components?.[0]?.id ?? "";
    setMappingForm((current) =>
      current.componentId || !firstComponent
        ? current
        : { ...current, componentId: firstComponent },
    );
  }, [detailResponse.data]);

  useEffect(() => {
    const incidents = detailResponse.data?.incidents ?? [];
    if (selectedIncidentId && incidents.some((incident) => incident.id === selectedIncidentId)) {
      return;
    }
    setSelectedIncidentId(incidents[0]?.id ?? "");
  }, [detailResponse.data, selectedIncidentId]);

  const detail = detailResponse.data;
  const detailPage = detail?.page ?? selectedPage;
  const incidents = detail?.incidents ?? [];
  const selectedIncident = incidents.find((incident) => incident.id === selectedIncidentId);

  useEffect(() => {
    setEditIncidentForm(incidentFormFromIncident(selectedIncident));
  }, [selectedIncident]);

  useEffect(() => {
    setPageSettingsForm(pageSettingsFormFromPage(detailPage));
  }, [detailPage]);

  const refreshStatusPages = () => {
    void pagesResponse.refetch();
    void detailResponse.refetch();
    void previewResponse.refetch();
  };

  const createPage = useCreateStatusPage({
    mutation: {
      onSuccess: (result) => {
        setPageForm(emptyPageForm);
        const createdId = result.page?.id;
        if (createdId) setSearchParams({ page: createdId });
        refreshStatusPages();
      },
    },
  });
  const createSection = useCreateStatusPageSection({
    mutation: {
      onSuccess: () => {
        setSectionForm(emptySectionForm);
        refreshStatusPages();
      },
    },
  });
  const createComponent = useCreateStatusPageComponent({
    mutation: {
      onSuccess: () => {
        setComponentForm((current) => ({ ...emptyComponentForm, sectionId: current.sectionId }));
        refreshStatusPages();
      },
    },
  });
  const createMapping = useCreateStatusPageComponentMapping({
    mutation: {
      onSuccess: () => {
        setMappingForm((current) => ({
          ...emptyMappingForm,
          componentId: current.componentId,
          resourceType: current.resourceType,
        }));
        refreshStatusPages();
      },
    },
  });
  const createIncident = useCreateStatusPageIncident({
    mutation: {
      onSuccess: (result) => {
        setCreateIncidentForm(emptyIncidentForm);
        if (result.incident?.id) setSelectedIncidentId(result.incident.id);
        refreshStatusPages();
      },
    },
  });
  const updateIncident = useUpdateStatusPageIncident({
    mutation: { onSuccess: refreshStatusPages },
  });
  const createIncidentUpdate = useCreateStatusPageIncidentUpdate({
    mutation: {
      onSuccess: () => {
        setUpdateForm(emptyIncidentUpdateForm);
        refreshStatusPages();
      },
    },
  });
  const updatePage = useUpdateStatusPage({ mutation: { onSuccess: refreshStatusPages } });
  const publishPage = usePublishStatusPage({ mutation: { onSuccess: refreshStatusPages } });
  const unpublishPage = useUnpublishStatusPage({ mutation: { onSuccess: refreshStatusPages } });

  const preview = previewResponse.data?.preview;
  const monitors = monitorsResponse.data?.monitors ?? [];
  const agents = agentsResponse.data?.agents ?? [];
  const internalIncidents = internalIncidentsResponse.data?.incidents ?? [];
  const createSuggestionIncidentId = createIncidentForm.internalIncidentId.trim();
  const editSuggestionIncidentId = editIncidentForm.internalIncidentId.trim();
  const createSuggestionsResponse = useSuggestStatusPageIncidentComponents(
    pageId,
    { incident_id: createSuggestionIncidentId },
    { query: { enabled: Boolean(pageId && createSuggestionIncidentId) } },
  );
  const editSuggestionsResponse = useSuggestStatusPageIncidentComponents(
    pageId,
    { incident_id: editSuggestionIncidentId },
    { query: { enabled: Boolean(pageId && editSuggestionIncidentId && selectedIncident) } },
  );
  const createSuggestions = createSuggestionsResponse.data?.suggestions ?? [];
  const editSuggestions = editSuggestionsResponse.data?.suggestions ?? [];
  const selectedResourceOptions =
    mappingForm.resourceType === "monitor"
      ? monitors.map((monitor) => ({
          id: monitor.id ?? "",
          label: monitor.name ?? monitor.id ?? "",
        }))
      : agents.map((agent) => ({ id: agent.id ?? "", label: agent.name ?? agent.id ?? "" }));

  const selectPage = (page: ApiStatusPageResponse) => {
    if (page.id) setSearchParams({ page: page.id });
  };

  const submitPage = (event: FormEvent) => {
    event.preventDefault();
    createPage.mutate({
      data: {
        description: pageForm.description.trim() || undefined,
        slug: pageForm.slug.trim(),
        title: pageForm.title.trim(),
      },
    });
  };

  const submitPageSettings = (event: FormEvent) => {
    event.preventDefault();
    if (!pageId || !detailPage) return;
    updatePage.mutate({
      id: pageId,
      data: {
        canonical_url: pageSettingsForm.canonicalUrl.trim(),
        default_incident_visibility: pageSettingsForm.defaultIncidentVisibility,
        description: pageSettingsForm.description.trim(),
        open_graph_image_url: pageSettingsForm.openGraphImageUrl.trim(),
        seo_description: pageSettingsForm.seoDescription.trim(),
        seo_title: pageSettingsForm.seoTitle.trim(),
        theme_settings: pageThemeSettings(detailPage.theme_settings, pageSettingsForm),
      },
    });
  };

  const submitSection = (event: FormEvent) => {
    event.preventDefault();
    if (!pageId) return;
    createSection.mutate({ id: pageId, data: { name: sectionForm.name.trim() } });
  };

  const submitComponent = (event: FormEvent) => {
    event.preventDefault();
    if (!pageId) return;
    createComponent.mutate({
      id: pageId,
      data: {
        display_mode: componentForm.manualStatus ? "manual" : "single_resource",
        manual_status: componentForm.manualStatus || undefined,
        public_description: componentForm.publicDescription.trim() || undefined,
        public_name: componentForm.publicName.trim(),
        section_id: componentForm.sectionId,
        visible: true,
      },
    });
  };

  const submitMapping = (event: FormEvent) => {
    event.preventDefault();
    if (!pageId || !mappingForm.componentId) return;
    createMapping.mutate({
      id: pageId,
      componentId: mappingForm.componentId,
      data: {
        health_rollup_strategy: "worst",
        resource_id: mappingForm.resourceId,
        resource_type: mappingForm.resourceType,
        uptime_rollup_strategy: "worst",
      },
    });
  };

  const toggleIncidentComponent = (
    form: IncidentFormState,
    setForm: (form: IncidentFormState) => void,
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

  const applySuggestedIncidentComponents = (
    form: IncidentFormState,
    setForm: (form: IncidentFormState) => void,
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

  const incidentRequest = (form: IncidentFormState) => ({
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

  const submitCreateIncident = (event: FormEvent) => {
    event.preventDefault();
    if (!pageId) return;
    createIncident.mutate({
      id: pageId,
      data: {
        ...incidentRequest(createIncidentForm),
        visibility: "draft",
      },
    });
  };

  const submitEditIncident = (event: FormEvent) => {
    event.preventDefault();
    if (!pageId || !selectedIncident?.id) return;
    updateIncident.mutate({
      id: pageId,
      incidentId: selectedIncident.id,
      data: incidentRequest(editIncidentForm),
    });
  };

  const publishIncident = () => {
    if (!pageId || !selectedIncident?.id) return;
    const form = {
      ...editIncidentForm,
      publishedAt: editIncidentForm.publishedAt || isoToDateTimeLocal(new Date().toISOString()),
      visibility: "published",
    };
    updateIncident.mutate({
      id: pageId,
      incidentId: selectedIncident.id,
      data: incidentRequest(form),
    });
  };

  const resolveIncident = () => {
    if (!pageId || !selectedIncident?.id) return;
    const now = isoToDateTimeLocal(new Date().toISOString());
    const form = {
      ...editIncidentForm,
      publicStatus: "resolved",
      publishedAt: editIncidentForm.publishedAt || now,
      resolvedAt: now,
      visibility: "published",
    };
    updateIncident.mutate({
      id: pageId,
      incidentId: selectedIncident.id,
      data: incidentRequest(form),
    });
  };

  const submitIncidentUpdate = (event: FormEvent) => {
    event.preventDefault();
    if (!pageId || !selectedIncident?.id) return;
    const publishedAt = dateTimeLocalToIso(updateForm.publishedAt) || new Date().toISOString();
    createIncidentUpdate.mutate({
      id: pageId,
      incidentId: selectedIncident.id,
      data: {
        created_by: updateForm.createdBy.trim() || undefined,
        message: updateForm.message.trim(),
        published_at: publishedAt,
        status: updateForm.status,
      },
    });
  };

  const renderIncidentSuggestions = ({
    internalIncidentId,
    isError,
    isLoading,
    onApply,
    suggestions,
  }: {
    internalIncidentId: string;
    isError: boolean;
    isLoading: boolean;
    onApply: () => void;
    suggestions: ApiStatusPageIncidentComponentSuggestionResponse[];
  }) => {
    if (!internalIncidentId) return null;

    return (
      <div className="space-y-2 border border-neutral-200 p-3 text-sm">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="font-medium">Suggested components</div>
          <Button
            disabled={isLoading || suggestions.length === 0}
            onClick={onApply}
            type="button"
            variant="outline"
          >
            Apply suggestions
          </Button>
        </div>
        {isLoading && <div className="text-neutral-600">Loading suggested components...</div>}
        {isError && <div>Unable to load suggested components.</div>}
        {!isLoading && !isError && suggestions.length === 0 && (
          <div className="text-neutral-600">No mapped public components found.</div>
        )}
        {suggestions.length > 0 && (
          <div className="space-y-1">
            {suggestions.map((suggestion) => (
              <div
                className="flex flex-wrap items-center justify-between gap-2"
                key={suggestion.component_id}
              >
                <span>{suggestion.component_name}</span>
                <span className="text-neutral-600">{suggestionMatchLabel(suggestion)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="space-y-6">
      <PageHeader title="Status Pages" description="Public availability pages and components." />

      <div className="grid gap-6 lg:grid-cols-[280px_minmax(0,1fr)]">
        <aside className="space-y-4">
          <form className="space-y-3" onSubmit={submitPage}>
            <h2 className="text-sm font-medium">New Page</h2>
            <Field label="Slug">
              <Input
                value={pageForm.slug}
                onChange={(event) =>
                  setPageForm((current) => ({ ...current, slug: event.target.value }))
                }
                placeholder="main-status"
              />
            </Field>
            <Field label="Title">
              <Input
                value={pageForm.title}
                onChange={(event) =>
                  setPageForm((current) => ({ ...current, title: event.target.value }))
                }
                placeholder="Main Status"
              />
            </Field>
            <Field label="Description">
              <Textarea
                value={pageForm.description}
                onChange={(event) =>
                  setPageForm((current) => ({ ...current, description: event.target.value }))
                }
                rows={3}
              />
            </Field>
            <Button className="w-full" disabled={createPage.isPending}>
              <Plus className="size-4" />
              {createPage.isPending ? "Creating..." : "Create page"}
            </Button>
            {createPage.isError && <p className="text-sm">Unable to create page.</p>}
          </form>

          <section className="space-y-2">
            <h2 className="text-sm font-medium">Pages</h2>
            {pagesResponse.isLoading && <div className="text-sm text-neutral-600">Loading...</div>}
            {pagesResponse.error && <div className="text-sm">Unable to load status pages.</div>}
            {pages.map((page) => (
              <button
                className={`block w-full border px-3 py-2 text-left text-sm ${
                  page.id === selectedPage?.id
                    ? "border-neutral-950 bg-neutral-100"
                    : "border-neutral-200 hover:bg-neutral-50"
                }`}
                key={page.id}
                onClick={() => selectPage(page)}
                type="button"
              >
                <span className="block font-medium">{page.title}</span>
                <span className="text-neutral-600">{page.slug}</span>
              </button>
            ))}
            {!pagesResponse.isLoading && pages.length === 0 && (
              <EmptyState title="No status pages" description="Create a draft page to start." />
            )}
          </section>
        </aside>

        {!selectedPage && (
          <EmptyState title="No page selected" description="Create or select a status page." />
        )}

        {selectedPage && (
          <main className="space-y-6">
            <section className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <div className="flex flex-wrap items-center gap-2">
                  <h2 className="text-lg font-semibold">{selectedPage.title}</h2>
                  <StatusBadge
                    fallback={selectedPage.visibility}
                    value={selectedPage.visibility === "public" ? "up" : "unknown"}
                  />
                </div>
                <div className="mt-1 flex flex-wrap items-center gap-3 text-sm text-neutral-600">
                  <span>{selectedPage.slug}</span>
                  {selectedPage.slug && (
                    <a
                      className="inline-flex items-center gap-1"
                      href={publicUrl(selectedPage.slug)}
                    >
                      <ExternalLink className="size-3.5" />
                      Public URL
                    </a>
                  )}
                </div>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button
                  disabled={publishPage.isPending || selectedPage.visibility === "public"}
                  onClick={() => selectedPage.id && publishPage.mutate({ id: selectedPage.id })}
                >
                  <Globe2 className="size-4" />
                  {publishPage.isPending ? "Publishing..." : "Publish"}
                </Button>
                <Button
                  disabled={unpublishPage.isPending || selectedPage.visibility !== "public"}
                  onClick={() => selectedPage.id && unpublishPage.mutate({ id: selectedPage.id })}
                  variant="outline"
                >
                  {unpublishPage.isPending ? "Unpublishing..." : "Unpublish"}
                </Button>
              </div>
              {publishPage.isError && (
                <div className="basis-full text-sm">
                  Unable to publish. Check visible components and mappings.
                </div>
              )}
            </section>

            <form className="space-y-4" onSubmit={submitPageSettings}>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <h3 className="text-sm font-medium">Page Settings</h3>
                <Button disabled={!pageId || updatePage.isPending} variant="outline">
                  <CheckCircle2 className="size-4" />
                  {updatePage.isPending ? "Saving..." : "Save settings"}
                </Button>
              </div>
              <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_320px]">
                <div className="space-y-3">
                  <Field label="Public description">
                    <Textarea
                      value={pageSettingsForm.description}
                      onChange={(event) =>
                        setPageSettingsForm((current) => ({
                          ...current,
                          description: event.target.value,
                        }))
                      }
                      rows={4}
                    />
                  </Field>
                  <Field label="Default incident visibility">
                    <select
                      className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                      value={pageSettingsForm.defaultIncidentVisibility}
                      onChange={(event) =>
                        setPageSettingsForm((current) => ({
                          ...current,
                          defaultIncidentVisibility: event.target.value,
                        }))
                      }
                    >
                      {incidentVisibilities.map((visibility) => (
                        <option key={visibility.value} value={visibility.value}>
                          {visibility.label}
                        </option>
                      ))}
                    </select>
                  </Field>
                  <Field label="Canonical URL">
                    <Input
                      placeholder="https://status.example.com"
                      type="url"
                      value={pageSettingsForm.canonicalUrl}
                      onChange={(event) =>
                        setPageSettingsForm((current) => ({
                          ...current,
                          canonicalUrl: event.target.value,
                        }))
                      }
                    />
                  </Field>
                </div>

                <div className="space-y-3">
                  <Field label="SEO title">
                    <Input
                      value={pageSettingsForm.seoTitle}
                      onChange={(event) =>
                        setPageSettingsForm((current) => ({
                          ...current,
                          seoTitle: event.target.value,
                        }))
                      }
                    />
                  </Field>
                  <Field label="SEO description">
                    <Textarea
                      value={pageSettingsForm.seoDescription}
                      onChange={(event) =>
                        setPageSettingsForm((current) => ({
                          ...current,
                          seoDescription: event.target.value,
                        }))
                      }
                      rows={3}
                    />
                  </Field>
                  <Field label="Open Graph image URL">
                    <Input
                      placeholder="https://status.example.com/og.png"
                      type="url"
                      value={pageSettingsForm.openGraphImageUrl}
                      onChange={(event) =>
                        setPageSettingsForm((current) => ({
                          ...current,
                          openGraphImageUrl: event.target.value,
                        }))
                      }
                    />
                  </Field>
                </div>

                <div className="space-y-3">
                  <div className="grid gap-3 sm:grid-cols-[96px_minmax(0,1fr)] xl:grid-cols-1">
                    <Field label="Accent color">
                      <Input
                        type="color"
                        value={pageSettingsForm.accentColor}
                        onChange={(event) =>
                          setPageSettingsForm((current) => ({
                            ...current,
                            accentColor: event.target.value,
                          }))
                        }
                      />
                    </Field>
                    <Field label="Logo URL">
                      <Input
                        placeholder="https://status.example.com/logo.svg"
                        type="url"
                        value={pageSettingsForm.logoUrl}
                        onChange={(event) =>
                          setPageSettingsForm((current) => ({
                            ...current,
                            logoUrl: event.target.value,
                          }))
                        }
                      />
                    </Field>
                  </div>
                  <Field label="Logo alt text">
                    <Input
                      value={pageSettingsForm.logoAlt}
                      onChange={(event) =>
                        setPageSettingsForm((current) => ({
                          ...current,
                          logoAlt: event.target.value,
                        }))
                      }
                    />
                  </Field>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <Field label="Header style">
                      <select
                        className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                        value={pageSettingsForm.headerStyle}
                        onChange={(event) =>
                          setPageSettingsForm((current) => ({
                            ...current,
                            headerStyle: event.target.value,
                          }))
                        }
                      >
                        {headerStyleOptions.map((option) => (
                          <option key={option.value} value={option.value}>
                            {option.label}
                          </option>
                        ))}
                      </select>
                    </Field>
                    <Field label="Component density">
                      <select
                        className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                        value={pageSettingsForm.componentDensity}
                        onChange={(event) =>
                          setPageSettingsForm((current) => ({
                            ...current,
                            componentDensity: event.target.value,
                          }))
                        }
                      >
                        {componentDensityOptions.map((option) => (
                          <option key={option.value} value={option.value}>
                            {option.label}
                          </option>
                        ))}
                      </select>
                    </Field>
                  </div>
                  <div className="grid gap-2 text-sm sm:grid-cols-2 xl:grid-cols-1">
                    <label className="flex items-center gap-2">
                      <input
                        checked={pageSettingsForm.showUptimeSummary}
                        onChange={(event) =>
                          setPageSettingsForm((current) => ({
                            ...current,
                            showUptimeSummary: event.target.checked,
                          }))
                        }
                        type="checkbox"
                      />
                      <span>Show uptime summary</span>
                    </label>
                    <label className="flex items-center gap-2">
                      <input
                        checked={pageSettingsForm.showIncidentHistory}
                        onChange={(event) =>
                          setPageSettingsForm((current) => ({
                            ...current,
                            showIncidentHistory: event.target.checked,
                          }))
                        }
                        type="checkbox"
                      />
                      <span>Show incident history</span>
                    </label>
                  </div>
                </div>
              </div>
              {updatePage.isError && <p className="text-sm">Unable to save page settings.</p>}
            </form>

            <section className="grid gap-4 xl:grid-cols-3">
              <form className="space-y-3" onSubmit={submitSection}>
                <h3 className="text-sm font-medium">Sections</h3>
                <Field label="Name">
                  <Input
                    value={sectionForm.name}
                    onChange={(event) => setSectionForm({ name: event.target.value })}
                    placeholder="API"
                  />
                </Field>
                <Button disabled={!pageId || createSection.isPending} variant="outline">
                  <Plus className="size-4" />
                  Add section
                </Button>
              </form>

              <form className="space-y-3" onSubmit={submitComponent}>
                <h3 className="text-sm font-medium">Components</h3>
                <Field label="Section">
                  <select
                    className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                    value={componentForm.sectionId}
                    onChange={(event) =>
                      setComponentForm((current) => ({ ...current, sectionId: event.target.value }))
                    }
                  >
                    <option value="">Select section</option>
                    {(detail?.sections ?? []).map((section) => (
                      <option key={section.id} value={section.id}>
                        {section.name}
                      </option>
                    ))}
                  </select>
                </Field>
                <Field label="Public name">
                  <Input
                    value={componentForm.publicName}
                    onChange={(event) =>
                      setComponentForm((current) => ({
                        ...current,
                        publicName: event.target.value,
                      }))
                    }
                    placeholder="REST API"
                  />
                </Field>
                <Field label="Description">
                  <Input
                    value={componentForm.publicDescription}
                    onChange={(event) =>
                      setComponentForm((current) => ({
                        ...current,
                        publicDescription: event.target.value,
                      }))
                    }
                  />
                </Field>
                <Field label="Manual status">
                  <select
                    className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                    value={componentForm.manualStatus}
                    onChange={(event) =>
                      setComponentForm((current) => ({
                        ...current,
                        manualStatus: event.target.value,
                      }))
                    }
                  >
                    {manualStatuses.map((status) => (
                      <option key={status.value} value={status.value}>
                        {status.label}
                      </option>
                    ))}
                  </select>
                </Field>
                <Button
                  disabled={!componentForm.sectionId || createComponent.isPending}
                  variant="outline"
                >
                  <Plus className="size-4" />
                  Add component
                </Button>
              </form>

              <form className="space-y-3" onSubmit={submitMapping}>
                <h3 className="text-sm font-medium">Mappings</h3>
                <Field label="Component">
                  <select
                    className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                    value={mappingForm.componentId}
                    onChange={(event) =>
                      setMappingForm((current) => ({ ...current, componentId: event.target.value }))
                    }
                  >
                    <option value="">Select component</option>
                    {(detail?.components ?? []).map((component) => (
                      <option key={component.id} value={component.id}>
                        {component.public_name}
                      </option>
                    ))}
                  </select>
                </Field>
                <Field label="Resource type">
                  <select
                    className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                    value={mappingForm.resourceType}
                    onChange={(event) =>
                      setMappingForm({
                        componentId: mappingForm.componentId,
                        resourceId: "",
                        resourceType: event.target.value as MappingFormState["resourceType"],
                      })
                    }
                  >
                    <option value="monitor">Monitor</option>
                    <option value="agent">Agent</option>
                  </select>
                </Field>
                <Field label="Resource">
                  <select
                    className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                    value={mappingForm.resourceId}
                    onChange={(event) =>
                      setMappingForm((current) => ({ ...current, resourceId: event.target.value }))
                    }
                  >
                    <option value="">Select resource</option>
                    {selectedResourceOptions.map((resource) => (
                      <option key={resource.id} value={resource.id}>
                        {resource.label}
                      </option>
                    ))}
                  </select>
                </Field>
                <Button
                  disabled={
                    !mappingForm.componentId || !mappingForm.resourceId || createMapping.isPending
                  }
                  variant="outline"
                >
                  <Link2 className="size-4" />
                  Add mapping
                </Button>
              </form>
            </section>

            <section className="grid gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
              <form className="space-y-3" onSubmit={submitCreateIncident}>
                <h3 className="text-sm font-medium">New Public Incident</h3>
                <Field label="Internal incident link">
                  <select
                    className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                    value={createIncidentForm.internalIncidentId}
                    onChange={(event) =>
                      setCreateIncidentForm((current) => ({
                        ...current,
                        internalIncidentId: event.target.value,
                      }))
                    }
                  >
                    <option value="">No linked incident</option>
                    {internalIncidents.map((incident) => (
                      <option key={incident.id} value={incident.id}>
                        {incidentOptionLabel(incident)}
                      </option>
                    ))}
                  </select>
                </Field>
                <Field label="Internal incident ID">
                  <Input
                    value={createIncidentForm.internalIncidentId}
                    onChange={(event) =>
                      setCreateIncidentForm((current) => ({
                        ...current,
                        internalIncidentId: event.target.value,
                      }))
                    }
                    placeholder="incident_..."
                  />
                </Field>
                {renderIncidentSuggestions({
                  internalIncidentId: createSuggestionIncidentId,
                  isError: createSuggestionsResponse.isError,
                  isLoading: createSuggestionsResponse.isLoading,
                  onApply: () =>
                    applySuggestedIncidentComponents(
                      createIncidentForm,
                      setCreateIncidentForm,
                      createSuggestions,
                    ),
                  suggestions: createSuggestions,
                })}
                <Field label="Public title">
                  <Input
                    value={createIncidentForm.title}
                    onChange={(event) =>
                      setCreateIncidentForm((current) => ({
                        ...current,
                        title: event.target.value,
                      }))
                    }
                    placeholder="API latency elevated"
                  />
                </Field>
                <div className="grid gap-3 sm:grid-cols-2">
                  <Field label="Public status">
                    <select
                      className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                      value={createIncidentForm.publicStatus}
                      onChange={(event) =>
                        setCreateIncidentForm((current) => ({
                          ...current,
                          publicStatus: event.target.value,
                        }))
                      }
                    >
                      {incidentStatuses.map((status) => (
                        <option key={status.value} value={status.value}>
                          {status.label}
                        </option>
                      ))}
                    </select>
                  </Field>
                  <Field label="Severity">
                    <select
                      className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                      value={createIncidentForm.severity}
                      onChange={(event) =>
                        setCreateIncidentForm((current) => ({
                          ...current,
                          severity: event.target.value,
                        }))
                      }
                    >
                      {incidentSeverities.map((severity) => (
                        <option key={severity.value} value={severity.value}>
                          {severity.label}
                        </option>
                      ))}
                    </select>
                  </Field>
                </div>
                <Field label="Public impact">
                  <Textarea
                    value={createIncidentForm.impactSummary}
                    onChange={(event) =>
                      setCreateIncidentForm((current) => ({
                        ...current,
                        impactSummary: event.target.value,
                      }))
                    }
                    rows={3}
                  />
                </Field>
                <div className="grid gap-3 sm:grid-cols-2">
                  <Field label="Scheduled start">
                    <Input
                      type="datetime-local"
                      value={createIncidentForm.scheduledStartAt}
                      onChange={(event) =>
                        setCreateIncidentForm((current) => ({
                          ...current,
                          scheduledStartAt: event.target.value,
                        }))
                      }
                    />
                  </Field>
                  <Field label="Scheduled end">
                    <Input
                      type="datetime-local"
                      value={createIncidentForm.scheduledEndAt}
                      onChange={(event) =>
                        setCreateIncidentForm((current) => ({
                          ...current,
                          scheduledEndAt: event.target.value,
                        }))
                      }
                    />
                  </Field>
                </div>
                <div className="space-y-2">
                  <div className="text-sm font-medium">Affected components</div>
                  <div className="space-y-1">
                    {(detail?.components ?? []).map((component) => (
                      <label className="flex items-center gap-2 text-sm" key={component.id}>
                        <input
                          checked={createIncidentForm.affectedComponentIds.includes(
                            component.id ?? "",
                          )}
                          onChange={() =>
                            component.id &&
                            toggleIncidentComponent(
                              createIncidentForm,
                              setCreateIncidentForm,
                              component.id,
                            )
                          }
                          type="checkbox"
                        />
                        <span>{component.public_name}</span>
                      </label>
                    ))}
                    {(detail?.components ?? []).length === 0 && (
                      <div className="text-sm text-neutral-600">No components configured.</div>
                    )}
                  </div>
                </div>
                <Button
                  disabled={!pageId || !createIncidentForm.title.trim() || createIncident.isPending}
                  variant="outline"
                >
                  <Plus className="size-4" />
                  {createIncident.isPending ? "Creating..." : "Create draft incident"}
                </Button>
                {createIncident.isError && (
                  <p className="text-sm">Unable to create public incident.</p>
                )}
              </form>

              <div className="space-y-4">
                <div className="space-y-3">
                  <h3 className="text-sm font-medium">Configured Incidents</h3>
                  {incidents.length === 0 && (
                    <EmptyState
                      title="No public incidents"
                      description="Create a draft public incident when customer-facing communication is needed."
                    />
                  )}
                  <div className="grid gap-2 md:grid-cols-2">
                    {incidents.map((incident) => (
                      <button
                        className={`border px-3 py-2 text-left text-sm ${
                          incident.id === selectedIncidentId
                            ? "border-neutral-950 bg-neutral-100"
                            : "border-neutral-200 hover:bg-neutral-50"
                        }`}
                        key={incident.id}
                        onClick={() => setSelectedIncidentId(incident.id ?? "")}
                        type="button"
                      >
                        <span className="flex items-center justify-between gap-2">
                          <span className="font-medium">{incident.title}</span>
                          <StatusBadge
                            fallback={incident.public_status}
                            value={incidentBadgeStatus(incident.public_status)}
                          />
                        </span>
                        <span className="mt-1 block text-neutral-600">
                          {incident.visibility}
                          {incident.internal_incident_id
                            ? ` - linked ${incident.internal_incident_id}`
                            : ""}
                        </span>
                      </button>
                    ))}
                  </div>
                </div>

                {selectedIncident && (
                  <div className="grid gap-4 xl:grid-cols-2">
                    <form className="space-y-3" onSubmit={submitEditIncident}>
                      <div className="flex items-center justify-between gap-2">
                        <h3 className="text-sm font-medium">Edit Public Incident</h3>
                        <StatusBadge
                          fallback={editIncidentForm.visibility}
                          value={editIncidentForm.visibility === "published" ? "up" : "unknown"}
                        />
                      </div>
                      <Field label="Internal incident ID">
                        <Input
                          value={editIncidentForm.internalIncidentId}
                          onChange={(event) =>
                            setEditIncidentForm((current) => ({
                              ...current,
                              internalIncidentId: event.target.value,
                            }))
                          }
                        />
                      </Field>
                      {renderIncidentSuggestions({
                        internalIncidentId: editSuggestionIncidentId,
                        isError: editSuggestionsResponse.isError,
                        isLoading: editSuggestionsResponse.isLoading,
                        onApply: () =>
                          applySuggestedIncidentComponents(
                            editIncidentForm,
                            setEditIncidentForm,
                            editSuggestions,
                          ),
                        suggestions: editSuggestions,
                      })}
                      <Field label="Public title">
                        <Input
                          value={editIncidentForm.title}
                          onChange={(event) =>
                            setEditIncidentForm((current) => ({
                              ...current,
                              title: event.target.value,
                            }))
                          }
                        />
                      </Field>
                      <div className="grid gap-3 sm:grid-cols-3">
                        <Field label="Public status">
                          <select
                            className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                            value={editIncidentForm.publicStatus}
                            onChange={(event) =>
                              setEditIncidentForm((current) => ({
                                ...current,
                                publicStatus: event.target.value,
                              }))
                            }
                          >
                            {incidentStatuses.map((status) => (
                              <option key={status.value} value={status.value}>
                                {status.label}
                              </option>
                            ))}
                          </select>
                        </Field>
                        <Field label="Severity">
                          <select
                            className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                            value={editIncidentForm.severity}
                            onChange={(event) =>
                              setEditIncidentForm((current) => ({
                                ...current,
                                severity: event.target.value,
                              }))
                            }
                          >
                            {incidentSeverities.map((severity) => (
                              <option key={severity.value} value={severity.value}>
                                {severity.label}
                              </option>
                            ))}
                          </select>
                        </Field>
                        <Field label="Visibility">
                          <select
                            className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                            value={editIncidentForm.visibility}
                            onChange={(event) =>
                              setEditIncidentForm((current) => ({
                                ...current,
                                visibility: event.target.value,
                              }))
                            }
                          >
                            {incidentVisibilities.map((visibility) => (
                              <option key={visibility.value} value={visibility.value}>
                                {visibility.label}
                              </option>
                            ))}
                          </select>
                        </Field>
                      </div>
                      <Field label="Public impact">
                        <Textarea
                          value={editIncidentForm.impactSummary}
                          onChange={(event) =>
                            setEditIncidentForm((current) => ({
                              ...current,
                              impactSummary: event.target.value,
                            }))
                          }
                          rows={3}
                        />
                      </Field>
                      <div className="grid gap-3 sm:grid-cols-2">
                        <Field label="Published at">
                          <Input
                            type="datetime-local"
                            value={editIncidentForm.publishedAt}
                            onChange={(event) =>
                              setEditIncidentForm((current) => ({
                                ...current,
                                publishedAt: event.target.value,
                              }))
                            }
                          />
                        </Field>
                        <Field label="Resolved at">
                          <Input
                            type="datetime-local"
                            value={editIncidentForm.resolvedAt}
                            onChange={(event) =>
                              setEditIncidentForm((current) => ({
                                ...current,
                                resolvedAt: event.target.value,
                              }))
                            }
                          />
                        </Field>
                      </div>
                      <div className="grid gap-3 sm:grid-cols-2">
                        <Field label="Scheduled start">
                          <Input
                            type="datetime-local"
                            value={editIncidentForm.scheduledStartAt}
                            onChange={(event) =>
                              setEditIncidentForm((current) => ({
                                ...current,
                                scheduledStartAt: event.target.value,
                              }))
                            }
                          />
                        </Field>
                        <Field label="Scheduled end">
                          <Input
                            type="datetime-local"
                            value={editIncidentForm.scheduledEndAt}
                            onChange={(event) =>
                              setEditIncidentForm((current) => ({
                                ...current,
                                scheduledEndAt: event.target.value,
                              }))
                            }
                          />
                        </Field>
                      </div>
                      <div className="space-y-2">
                        <div className="text-sm font-medium">Affected components</div>
                        <div className="grid gap-1 sm:grid-cols-2">
                          {(detail?.components ?? []).map((component) => (
                            <label className="flex items-center gap-2 text-sm" key={component.id}>
                              <input
                                checked={editIncidentForm.affectedComponentIds.includes(
                                  component.id ?? "",
                                )}
                                onChange={() =>
                                  component.id &&
                                  toggleIncidentComponent(
                                    editIncidentForm,
                                    setEditIncidentForm,
                                    component.id,
                                  )
                                }
                                type="checkbox"
                              />
                              <span>{component.public_name}</span>
                            </label>
                          ))}
                        </div>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <Button
                          disabled={!editIncidentForm.title.trim() || updateIncident.isPending}
                          variant="outline"
                        >
                          {updateIncident.isPending ? "Saving..." : "Save incident"}
                        </Button>
                        <Button
                          disabled={updateIncident.isPending}
                          onClick={publishIncident}
                          type="button"
                          variant="outline"
                        >
                          <Globe2 className="size-4" />
                          Publish
                        </Button>
                        <Button
                          disabled={updateIncident.isPending}
                          onClick={resolveIncident}
                          type="button"
                          variant="outline"
                        >
                          <CheckCircle2 className="size-4" />
                          Resolve
                        </Button>
                      </div>
                      {updateIncident.isError && (
                        <p className="text-sm">Unable to update public incident.</p>
                      )}
                    </form>

                    <div className="space-y-4">
                      <form className="space-y-3" onSubmit={submitIncidentUpdate}>
                        <h3 className="text-sm font-medium">Add Public Update</h3>
                        <div className="grid gap-3 sm:grid-cols-2">
                          <Field label="Status">
                            <select
                              className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                              value={updateForm.status}
                              onChange={(event) =>
                                setUpdateForm((current) => ({
                                  ...current,
                                  status: event.target.value,
                                }))
                              }
                            >
                              {incidentStatuses.map((status) => (
                                <option key={status.value} value={status.value}>
                                  {status.label}
                                </option>
                              ))}
                            </select>
                          </Field>
                          <Field label="Published at">
                            <Input
                              type="datetime-local"
                              value={updateForm.publishedAt}
                              onChange={(event) =>
                                setUpdateForm((current) => ({
                                  ...current,
                                  publishedAt: event.target.value,
                                }))
                              }
                            />
                          </Field>
                        </div>
                        <Field label="Message">
                          <Textarea
                            value={updateForm.message}
                            onChange={(event) =>
                              setUpdateForm((current) => ({
                                ...current,
                                message: event.target.value,
                              }))
                            }
                            rows={4}
                          />
                        </Field>
                        <Field label="Created by">
                          <Input
                            value={updateForm.createdBy}
                            onChange={(event) =>
                              setUpdateForm((current) => ({
                                ...current,
                                createdBy: event.target.value,
                              }))
                            }
                            placeholder="Support"
                          />
                        </Field>
                        <Button
                          disabled={!updateForm.message.trim() || createIncidentUpdate.isPending}
                        >
                          <Plus className="size-4" />
                          {createIncidentUpdate.isPending ? "Adding..." : "Add update"}
                        </Button>
                        {createIncidentUpdate.isError && (
                          <p className="text-sm">Unable to add public update.</p>
                        )}
                      </form>

                      <div className="space-y-2">
                        <h3 className="text-sm font-medium">Public Updates</h3>
                        {(selectedIncident.updates ?? []).length === 0 && (
                          <div className="text-sm text-neutral-600">No updates yet.</div>
                        )}
                        {(selectedIncident.updates ?? []).map((update) => (
                          <div className="border border-neutral-200 p-3 text-sm" key={update.id}>
                            <div className="flex flex-wrap items-center justify-between gap-2">
                              <StatusBadge
                                fallback={update.status}
                                value={incidentBadgeStatus(update.status)}
                              />
                              <span className="text-neutral-600">
                                {formatDateTime(update.published_at ?? update.created_at)}
                              </span>
                            </div>
                            <p className="mt-2 whitespace-pre-wrap">{update.message}</p>
                            {update.created_by && (
                              <div className="mt-2 text-neutral-600">By {update.created_by}</div>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                )}
              </div>
            </section>

            <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
              <div className="space-y-3">
                <h3 className="text-sm font-medium">Configured Components</h3>
                {(detail?.components ?? []).length === 0 && (
                  <EmptyState title="No components" description="Add a section and component." />
                )}
                {(detail?.components ?? []).map((component) => (
                  <div className="border border-neutral-200 p-3" key={component.id}>
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div>
                        <div className="font-medium">{component.public_name}</div>
                        <div className="text-sm text-neutral-600">
                          {component.public_description}
                        </div>
                      </div>
                      <StatusBadge
                        fallback={component.manual_status || component.display_mode}
                        value={statusBadgeStatus(component.manual_status)}
                      />
                    </div>
                    <div className="mt-3 space-y-1 text-sm">
                      {(component.mappings ?? []).map((mapping) => (
                        <div className="flex items-center gap-2" key={mapping.id}>
                          <RadioTower className="size-3.5 text-neutral-500" />
                          <span>{mapping.resource_type}</span>
                          <span className="text-neutral-600">{mapping.resource_id}</span>
                        </div>
                      ))}
                      {(component.mappings ?? []).length === 0 && (
                        <div className="text-neutral-600">No mappings</div>
                      )}
                    </div>
                  </div>
                ))}
              </div>

              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <h3 className="text-sm font-medium">Preview</h3>
                  <Eye className="size-4 text-neutral-500" />
                </div>
                {previewResponse.isLoading && (
                  <div className="text-sm text-neutral-600">Loading...</div>
                )}
                {preview && (
                  <div className="border border-neutral-200 p-3">
                    <div className="flex items-center justify-between gap-2">
                      <div>
                        <div className="font-medium">{preview.page?.title}</div>
                        <div className="text-sm text-neutral-600">{preview.page?.slug}</div>
                      </div>
                      <StatusBadge
                        fallback={preview.overall_status}
                        value={statusBadgeStatus(preview.overall_status)}
                      />
                    </div>
                    <div className="mt-4 space-y-3">
                      {(preview.sections ?? []).map((section) => (
                        <div key={section.id}>
                          <div className="text-sm font-medium">{section.name}</div>
                          <div className="mt-2 space-y-2">
                            {(section.components ?? []).map(
                              (component: ApiStatusPagePublicComponentResponse) => (
                                <div
                                  className="flex items-center justify-between text-sm"
                                  key={component.id}
                                >
                                  <span>{component.name}</span>
                                  <StatusBadge
                                    fallback={component.status}
                                    value={statusBadgeStatus(component.status)}
                                  />
                                </div>
                              ),
                            )}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </section>
          </main>
        )}
      </div>
    </div>
  );
};
