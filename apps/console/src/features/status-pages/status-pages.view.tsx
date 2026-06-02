import { EmptyState } from "@/components/shared/empty-state";
import { PageHeader } from "@/components/shared/page-header";
import {
  type ApiStatusPageResponse,
  useCreateStatusPage,
  useCreateStatusPageComponent,
  useCreateStatusPageComponentMapping,
  useCreateStatusPageIncident,
  useCreateStatusPageIncidentUpdate,
  useCreateStatusPageSection,
  useDeleteStatusPage,
  useDeleteStatusPageComponent,
  useDeleteStatusPageComponentMapping,
  useDeleteStatusPageIncident,
  useDeleteStatusPageSection,
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
import { type FormEvent, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { deriveStatusPageViewState } from "./status-pages.domain";
import { StatusPageHeader, StatusPageTabSwitch } from "./components/status-pages-header";
import { StatusPagesSetupTab } from "./components/status-pages-setup-tab";
import { StatusPagesSidebar } from "./components/status-pages-sidebar";
import { StatusPageSubscribersTab } from "./components/status-pages-subscribers-tab";
import {
  dateTimeLocalToIso,
  emptyComponentForm,
  emptyIncidentForm,
  emptyIncidentUpdateForm,
  emptyMappingForm,
  emptyPageForm,
  emptyPageSettingsForm,
  emptySectionForm,
  incidentFormFromIncident,
  incidentRequest,
  isoToDateTimeLocal,
  pageSettingsFormFromPage,
  pageThemeSettings,
} from "./components/status-pages-shared";
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
    status: "open,acknowledged,covered,resolved",
  });
  const [pageForm, setPageForm] = useState(emptyPageForm);
  const [pageSettingsForm, setPageSettingsForm] = useState(emptyPageSettingsForm);
  const [sectionForm, setSectionForm] = useState(emptySectionForm);
  const [componentForm, setComponentForm] = useState(emptyComponentForm);
  const [mappingForm, setMappingForm] = useState(emptyMappingForm);
  const [createIncidentForm, setCreateIncidentForm] = useState(emptyIncidentForm);
  const [editIncidentForm, setEditIncidentForm] = useState(emptyIncidentForm);
  const [updateForm, setUpdateForm] = useState(emptyIncidentUpdateForm);
  const [selectedIncidentId, setSelectedIncidentId] = useState("");
  const [activePageTab, setActivePageTab] = useState<"setup" | "subscribers">("setup");
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
    const pageIncidents = detailResponse.data?.incidents ?? [];
    if (
      selectedIncidentId &&
      pageIncidents.some((incident) => incident.id === selectedIncidentId)
    ) {
      return;
    }
    setSelectedIncidentId(pageIncidents[0]?.id ?? "");
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
  const deletePage = useDeleteStatusPage({
    mutation: {
      onSuccess: () => {
        setSearchParams({});
        void pagesResponse.refetch();
      },
    },
  });
  const deleteSection = useDeleteStatusPageSection({
    mutation: { onSuccess: refreshStatusPages },
  });
  const deleteComponent = useDeleteStatusPageComponent({
    mutation: { onSuccess: refreshStatusPages },
  });
  const deleteMapping = useDeleteStatusPageComponentMapping({
    mutation: { onSuccess: refreshStatusPages },
  });
  const deleteIncident = useDeleteStatusPageIncident({
    mutation: {
      onSuccess: () => {
        setSelectedIncidentId("");
        refreshStatusPages();
      },
    },
  });
  const preview = previewResponse.data?.preview;
  const sections = detail?.sections ?? [];
  const components = detail?.components ?? [];
  const {
    canPublish,
    deletePending,
    previewDark,
    publishBlockers,
    publishWarnings,
    resourceOptions,
  } = deriveStatusPageViewState({
    agents: agentsResponse.data?.agents ?? [],
    components,
    deleteFlags: [
      deletePage.isPending,
      deleteSection.isPending,
      deleteComponent.isPending,
      deleteMapping.isPending,
      deleteIncident.isPending,
    ],
    incidents,
    mappingResourceType: mappingForm.resourceType,
    monitors: monitorsResponse.data?.monitors ?? [],
    previewError: Boolean(previewResponse.error),
    previewThemeSettings: preview?.page?.theme_settings,
    sections,
    selectedPageVisibility: selectedPage?.visibility,
  });
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
  const selectPage = (page: ApiStatusPageResponse) => page.id && setSearchParams({ page: page.id });
  const removePage = () => {
    if (!selectedPage?.id) return;
    if (
      !window.confirm(`Delete ${selectedPage.title ?? "this status page"} and all nested data?`)
    ) {
      return;
    }
    deletePage.mutate({ id: selectedPage.id });
  };
  const removeSection = (sectionId?: string, label?: string) => {
    if (!pageId || !sectionId) return;
    if (!window.confirm(`Delete ${label ?? "this section"} and its components?`)) return;
    deleteSection.mutate({ id: pageId, sectionId });
  };
  const removeComponent = (componentId?: string, label?: string) => {
    if (!pageId || !componentId) return;
    if (!window.confirm(`Delete ${label ?? "this component"} and its mappings?`)) return;
    deleteComponent.mutate({ id: pageId, componentId });
  };
  const removeMapping = (componentId?: string, mappingId?: string) => {
    if (!pageId || !componentId || !mappingId) return;
    if (!window.confirm("Delete this component mapping?")) return;
    deleteMapping.mutate({ id: pageId, componentId, mappingId });
  };
  const removeIncident = () => {
    if (!pageId || !selectedIncident?.id) return;
    if (!window.confirm(`Delete ${selectedIncident.title ?? "this public incident"}?`)) return;
    deleteIncident.mutate({ id: pageId, incidentId: selectedIncident.id });
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
  const submitCreateIncident = (event: FormEvent) => {
    event.preventDefault();
    if (!pageId) return;
    createIncident.mutate({
      id: pageId,
      data: { ...incidentRequest(createIncidentForm), visibility: "draft" },
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
    updateIncident.mutate({
      id: pageId,
      incidentId: selectedIncident.id,
      data: incidentRequest({
        ...editIncidentForm,
        publishedAt: editIncidentForm.publishedAt || isoToDateTimeLocal(new Date().toISOString()),
        visibility: "published",
      }),
    });
  };
  const resolveIncident = () => {
    if (!pageId || !selectedIncident?.id) return;
    const now = isoToDateTimeLocal(new Date().toISOString());
    updateIncident.mutate({
      id: pageId,
      incidentId: selectedIncident.id,
      data: incidentRequest({
        ...editIncidentForm,
        publicStatus: "resolved",
        publishedAt: editIncidentForm.publishedAt || now,
        resolvedAt: now,
        visibility: "published",
      }),
    });
  };
  const submitIncidentUpdate = (event: FormEvent) => {
    event.preventDefault();
    if (!pageId || !selectedIncident?.id) return;
    createIncidentUpdate.mutate({
      id: pageId,
      incidentId: selectedIncident.id,
      data: {
        created_by: updateForm.createdBy.trim() || undefined,
        message: updateForm.message.trim(),
        published_at: dateTimeLocalToIso(updateForm.publishedAt) || new Date().toISOString(),
        status: updateForm.status,
      },
    });
  };
  return (
    <div className="space-y-6">
      <PageHeader title="Status Pages" description="Public availability pages and components." />
      <div className="grid gap-6 lg:grid-cols-[280px_minmax(0,1fr)]">
        <StatusPagesSidebar
          createPageError={createPage.isError}
          createPagePending={createPage.isPending}
          onSelectPage={selectPage}
          onSubmitPage={submitPage}
          pageForm={pageForm}
          pages={pages}
          pagesError={Boolean(pagesResponse.error)}
          pagesLoading={pagesResponse.isLoading}
          selectedPageId={selectedPage?.id}
          setPageForm={setPageForm}
        />
        {!selectedPage && (
          <EmptyState title="No page selected" description="Create or select a status page." />
        )}
        {selectedPage && (
          <main className="space-y-6">
            <StatusPageHeader
              canPublish={canPublish}
              deletePagePending={deletePage.isPending}
              deletePending={deletePending}
              onDeletePage={removePage}
              onPublishPage={() => selectedPage.id && publishPage.mutate({ id: selectedPage.id })}
              onUnpublishPage={() =>
                selectedPage.id && unpublishPage.mutate({ id: selectedPage.id })
              }
              page={selectedPage}
              publishBlockers={publishBlockers}
              publishError={publishPage.isError}
              publishPending={publishPage.isPending}
              publishWarnings={publishWarnings}
              unpublishPending={unpublishPage.isPending}
            />
            <StatusPageTabSwitch activePageTab={activePageTab} onChange={setActivePageTab} />
            {activePageTab === "subscribers" ? (
              <StatusPageSubscribersTab pageId={pageId} />
            ) : (
              <StatusPagesSetupTab
                componentForm={componentForm}
                components={components}
                createComponentPending={createComponent.isPending}
                createForm={createIncidentForm}
                createIncidentError={createIncident.isError}
                createIncidentPending={createIncident.isPending}
                createMappingPending={createMapping.isPending}
                createSectionPending={createSection.isPending}
                createSuggestions={createSuggestionsResponse.data?.suggestions ?? []}
                createSuggestionsError={createSuggestionsResponse.isError}
                createSuggestionsLoading={createSuggestionsResponse.isLoading}
                createUpdateError={createIncidentUpdate.isError}
                createUpdatePending={createIncidentUpdate.isPending}
                deleteIncidentPending={deleteIncident.isPending}
                deletePending={deletePending}
                editForm={editIncidentForm}
                editSuggestions={editSuggestionsResponse.data?.suggestions ?? []}
                editSuggestionsError={editSuggestionsResponse.isError}
                editSuggestionsLoading={editSuggestionsResponse.isLoading}
                incidents={incidents}
                internalIncidents={internalIncidentsResponse.data?.incidents ?? []}
                mappingForm={mappingForm}
                onDeleteIncident={removeIncident}
                onPublishIncident={publishIncident}
                onRemoveComponent={removeComponent}
                onRemoveMapping={removeMapping}
                onRemoveSection={removeSection}
                onResolveIncident={resolveIncident}
                onSelectIncident={setSelectedIncidentId}
                onSubmitComponent={submitComponent}
                onSubmitCreateIncident={submitCreateIncident}
                onSubmitEditIncident={submitEditIncident}
                onSubmitIncidentUpdate={submitIncidentUpdate}
                onSubmitMapping={submitMapping}
                onSubmitPageSettings={submitPageSettings}
                onSubmitSection={submitSection}
                pageId={pageId}
                pageSettingsForm={pageSettingsForm}
                preview={preview}
                previewDark={previewDark}
                previewError={Boolean(previewResponse.error)}
                previewLoading={previewResponse.isLoading}
                resourceOptions={resourceOptions}
                sectionForm={sectionForm}
                sections={sections}
                selectedIncident={selectedIncident}
                selectedIncidentId={selectedIncidentId}
                setComponentForm={setComponentForm}
                setCreateForm={setCreateIncidentForm}
                setEditForm={setEditIncidentForm}
                setMappingForm={setMappingForm}
                setPageSettingsForm={setPageSettingsForm}
                setSectionForm={setSectionForm}
                setUpdateForm={setUpdateForm}
                updateForm={updateForm}
                updateIncidentError={updateIncident.isError}
                updateIncidentPending={updateIncident.isPending}
                updatePageError={updatePage.isError}
                updatePagePending={updatePage.isPending}
              />
            )}
          </main>
        )}
      </div>
    </div>
  );
};
