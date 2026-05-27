import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { StatusBadge } from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  type ApiStatusPagePublicComponentResponse,
  type ApiStatusPageResponse,
  useCreateStatusPage,
  useCreateStatusPageComponent,
  useCreateStatusPageComponentMapping,
  useCreateStatusPageSection,
  useGetAgents,
  useGetMonitors,
  useGetStatusPage,
  useListStatusPages,
  usePreviewStatusPage,
  usePublishStatusPage,
  useUnpublishStatusPage,
} from "@/orion-sdk";
import { ExternalLink, Eye, Globe2, Link2, Plus, RadioTower } from "lucide-react";
import { type FormEvent, type ReactNode, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";

type PageFormState = {
  slug: string;
  title: string;
  description: string;
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

const emptyPageForm: PageFormState = {
  slug: "",
  title: "",
  description: "",
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

const manualStatuses = [
  { label: "No override", value: "" },
  { label: "Operational", value: "operational" },
  { label: "Degraded", value: "degraded" },
  { label: "Partial outage", value: "partial_outage" },
  { label: "Major outage", value: "major_outage" },
  { label: "Maintenance", value: "maintenance" },
  { label: "Unknown", value: "unknown" },
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

const publicUrl = (slug?: string) => (slug ? `/status/${slug}` : "");

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
  const [pageForm, setPageForm] = useState<PageFormState>(emptyPageForm);
  const [sectionForm, setSectionForm] = useState<SectionFormState>(emptySectionForm);
  const [componentForm, setComponentForm] = useState<ComponentFormState>(emptyComponentForm);
  const [mappingForm, setMappingForm] = useState<MappingFormState>(emptyMappingForm);

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
  const publishPage = usePublishStatusPage({ mutation: { onSuccess: refreshStatusPages } });
  const unpublishPage = useUnpublishStatusPage({ mutation: { onSuccess: refreshStatusPages } });

  const detail = detailResponse.data;
  const preview = previewResponse.data?.preview;
  const monitors = monitorsResponse.data?.monitors ?? [];
  const agents = agentsResponse.data?.agents ?? [];
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
