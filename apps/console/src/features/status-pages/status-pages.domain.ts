type ResourceOption = {
  id: string;
  label: string;
};

export const deriveStatusPageViewState = ({
  agents,
  components,
  deleteFlags,
  incidents,
  mappingResourceType,
  monitors,
  previewError,
  previewThemeSettings,
  sections,
  selectedPageVisibility,
}: {
  agents: { id?: string; name?: string }[];
  components: { mappings?: unknown[] }[];
  deleteFlags: boolean[];
  incidents: { visibility?: string }[];
  mappingResourceType: "monitor" | "agent";
  monitors: { id?: string; name?: string }[];
  previewError: boolean;
  previewThemeSettings?: Record<string, unknown>;
  sections: unknown[];
  selectedPageVisibility?: string;
}) => {
  const unpublishedIncidents = incidents.filter((incident) => incident.visibility !== "published");
  const publishedIncidents = incidents.filter((incident) => incident.visibility === "published");
  const unmappedComponents = components.filter(
    (component) => (component.mappings ?? []).length === 0,
  );
  const publishBlockers = [
    ...(sections.length === 0 ? ["Add at least one section before publishing."] : []),
    ...(components.length === 0 ? ["Add at least one public component before publishing."] : []),
    ...(unmappedComponents.length > 0
      ? [
          `Map ${unmappedComponents.length} public component${unmappedComponents.length === 1 ? "" : "s"} before publishing.`,
        ]
      : []),
  ];
  const publishWarnings = [
    ...(unpublishedIncidents.length > 0
      ? [
          `${unpublishedIncidents.length} public incident draft${unpublishedIncidents.length === 1 ? "" : "s"} will not appear on the public page.`,
        ]
      : []),
    ...(publishedIncidents.length === 0
      ? ["No public incidents are published. This is fine for a healthy page."]
      : []),
    ...(previewError ? ["Public preview could not be loaded before publishing."] : []),
  ];
  const resources = mappingResourceType === "monitor" ? monitors : agents;
  const resourceOptions: ResourceOption[] = resources
    .filter((resource): resource is { id: string; name?: string } => Boolean(resource.id?.trim()))
    .map((resource) => ({
      id: resource.id,
      label: resource.name ?? resource.id,
    }));
  const previewThemeMode = themeSettingString(previewThemeSettings, "theme_mode", "light");

  return {
    canPublish:
      publishBlockers.length === 0 && components.length > 0 && selectedPageVisibility !== "public",
    deletePending: deleteFlags.some(Boolean),
    previewDark: previewThemeMode === "dark",
    publishBlockers,
    publishWarnings,
    resourceOptions,
  };
};

const themeSettingString = (
  settings: Record<string, unknown> | undefined,
  key: string,
  fallback = "",
) => {
  const value = settings?.[key];
  return typeof value === "string" ? value : fallback;
};
