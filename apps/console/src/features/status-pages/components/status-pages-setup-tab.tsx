import type {
  ApiIncidentResponse,
  ApiStatusPageComponentResponse,
  ApiStatusPageIncidentComponentSuggestionResponse,
  ApiStatusPageIncidentResponse,
  ApiStatusPageSectionResponse,
} from "@/orion-sdk";
import type { ComponentProps, FormEvent } from "react";
import { StatusPageComponentsPanel } from "./status-pages-components-panel";
import { StatusPageIncidentsPanel } from "./status-pages-incidents-panel";
import { StatusPagePreviewPanel } from "./status-pages-preview-panel";
import { StatusPageSettingsForm } from "./status-pages-settings-form";
import { StatusPageSetupForms } from "./status-pages-setup-forms";
import type {
  ComponentFormState,
  IncidentFormState,
  IncidentUpdateFormState,
  MappingFormState,
  PageSettingsFormState,
  SectionFormState,
  StateSetter,
} from "./status-pages-shared";

type StatusPagesSetupTabProps = {
  componentForm: ComponentFormState;
  components: ApiStatusPageComponentResponse[];
  createComponentPending: boolean;
  createForm: IncidentFormState;
  createIncidentError: boolean;
  createIncidentPending: boolean;
  createMappingPending: boolean;
  createSectionPending: boolean;
  createSuggestions: ApiStatusPageIncidentComponentSuggestionResponse[];
  createSuggestionsError: boolean;
  createSuggestionsLoading: boolean;
  createUpdateError: boolean;
  createUpdatePending: boolean;
  deleteIncidentPending: boolean;
  deletePending: boolean;
  editForm: IncidentFormState;
  editSuggestions: ApiStatusPageIncidentComponentSuggestionResponse[];
  editSuggestionsError: boolean;
  editSuggestionsLoading: boolean;
  incidents: ApiStatusPageIncidentResponse[];
  internalIncidents: ApiIncidentResponse[];
  mappingForm: MappingFormState;
  onDeleteIncident: () => void;
  onPublishIncident: () => void;
  onRemoveComponent: (componentId?: string, label?: string) => void;
  onRemoveMapping: (componentId?: string, mappingId?: string) => void;
  onRemoveSection: (sectionId?: string, label?: string) => void;
  onResolveIncident: () => void;
  onSelectIncident: (id: string) => void;
  onSubmitComponent: (event: FormEvent) => void;
  onSubmitCreateIncident: (event: FormEvent) => void;
  onSubmitEditIncident: (event: FormEvent) => void;
  onSubmitIncidentUpdate: (event: FormEvent) => void;
  onSubmitMapping: (event: FormEvent) => void;
  onSubmitPageSettings: (event: FormEvent) => void;
  onSubmitSection: (event: FormEvent) => void;
  pageId: string;
  pageSettingsForm: PageSettingsFormState;
  preview: ComponentProps<typeof StatusPagePreviewPanel>["preview"];
  previewDark: boolean;
  previewError: boolean;
  previewLoading: boolean;
  resourceOptions: { id: string; label: string }[];
  sectionForm: SectionFormState;
  sections: ApiStatusPageSectionResponse[];
  selectedIncident?: ApiStatusPageIncidentResponse;
  selectedIncidentId: string;
  setComponentForm: StateSetter<ComponentFormState>;
  setCreateForm: StateSetter<IncidentFormState>;
  setEditForm: StateSetter<IncidentFormState>;
  setMappingForm: StateSetter<MappingFormState>;
  setPageSettingsForm: StateSetter<PageSettingsFormState>;
  setSectionForm: StateSetter<SectionFormState>;
  setUpdateForm: StateSetter<IncidentUpdateFormState>;
  updateForm: IncidentUpdateFormState;
  updateIncidentError: boolean;
  updateIncidentPending: boolean;
  updatePageError: boolean;
  updatePagePending: boolean;
};

export const StatusPagesSetupTab = ({
  componentForm,
  components,
  createComponentPending,
  createForm,
  createIncidentError,
  createIncidentPending,
  createMappingPending,
  createSectionPending,
  createSuggestions,
  createSuggestionsError,
  createSuggestionsLoading,
  createUpdateError,
  createUpdatePending,
  deleteIncidentPending,
  deletePending,
  editForm,
  editSuggestions,
  editSuggestionsError,
  editSuggestionsLoading,
  incidents,
  internalIncidents,
  mappingForm,
  onDeleteIncident,
  onPublishIncident,
  onRemoveComponent,
  onRemoveMapping,
  onRemoveSection,
  onResolveIncident,
  onSelectIncident,
  onSubmitComponent,
  onSubmitCreateIncident,
  onSubmitEditIncident,
  onSubmitIncidentUpdate,
  onSubmitMapping,
  onSubmitPageSettings,
  onSubmitSection,
  pageId,
  pageSettingsForm,
  preview,
  previewDark,
  previewError,
  previewLoading,
  resourceOptions,
  sectionForm,
  sections,
  selectedIncident,
  selectedIncidentId,
  setComponentForm,
  setCreateForm,
  setEditForm,
  setMappingForm,
  setPageSettingsForm,
  setSectionForm,
  setUpdateForm,
  updateForm,
  updateIncidentError,
  updateIncidentPending,
  updatePageError,
  updatePagePending,
}: StatusPagesSetupTabProps) => (
  <div className="space-y-6">
    <StatusPageSettingsForm
      form={pageSettingsForm}
      isError={updatePageError}
      isPending={updatePagePending}
      onSubmit={onSubmitPageSettings}
      pageId={pageId}
      setForm={setPageSettingsForm}
    />
    <StatusPageSetupForms
      componentForm={componentForm}
      components={components}
      createComponentPending={createComponentPending}
      createMappingPending={createMappingPending}
      createSectionPending={createSectionPending}
      deletePending={deletePending}
      mappingForm={mappingForm}
      onRemoveSection={onRemoveSection}
      onSubmitComponent={onSubmitComponent}
      onSubmitMapping={onSubmitMapping}
      onSubmitSection={onSubmitSection}
      pageId={pageId}
      resourceOptions={resourceOptions}
      sectionForm={sectionForm}
      sections={sections}
      setComponentForm={setComponentForm}
      setMappingForm={setMappingForm}
      setSectionForm={setSectionForm}
    />
    <StatusPageIncidentsPanel
      components={components}
      createForm={createForm}
      createIncidentError={createIncidentError}
      createIncidentPending={createIncidentPending}
      createSuggestions={createSuggestions}
      createSuggestionsError={createSuggestionsError}
      createSuggestionsLoading={createSuggestionsLoading}
      createUpdateError={createUpdateError}
      createUpdatePending={createUpdatePending}
      deleteIncidentPending={deleteIncidentPending}
      deletePending={deletePending}
      editForm={editForm}
      editSuggestions={editSuggestions}
      editSuggestionsError={editSuggestionsError}
      editSuggestionsLoading={editSuggestionsLoading}
      incidents={incidents}
      internalIncidents={internalIncidents}
      onDeleteIncident={onDeleteIncident}
      onPublishIncident={onPublishIncident}
      onResolveIncident={onResolveIncident}
      onSelectIncident={onSelectIncident}
      onSubmitCreateIncident={onSubmitCreateIncident}
      onSubmitEditIncident={onSubmitEditIncident}
      onSubmitIncidentUpdate={onSubmitIncidentUpdate}
      pageId={pageId}
      selectedIncident={selectedIncident}
      selectedIncidentId={selectedIncidentId}
      setCreateForm={setCreateForm}
      setEditForm={setEditForm}
      setUpdateForm={setUpdateForm}
      updateForm={updateForm}
      updateIncidentError={updateIncidentError}
      updateIncidentPending={updateIncidentPending}
    />
    <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
      <StatusPageComponentsPanel
        components={components}
        deletePending={deletePending}
        onRemoveComponent={onRemoveComponent}
        onRemoveMapping={onRemoveMapping}
      />
      <StatusPagePreviewPanel
        isError={previewError}
        isLoading={previewLoading}
        preview={preview}
        previewDark={previewDark}
      />
    </section>
  </div>
);
