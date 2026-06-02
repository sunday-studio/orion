import { EmptyState } from "@/components/shared/empty-state";
import { StatusBadge } from "@/components/shared/status-badges";
import type {
  ApiIncidentResponse,
  ApiStatusPageComponentResponse,
  ApiStatusPageIncidentComponentSuggestionResponse,
  ApiStatusPageIncidentResponse,
} from "@/orion-sdk";
import type { FormEvent } from "react";
import { CreateIncidentForm } from "./status-pages-create-incident-form";
import { EditIncidentForm } from "./status-pages-edit-incident-form";
import { IncidentUpdatesPanel } from "./status-pages-incident-updates";
import {
  type IncidentFormState,
  type IncidentUpdateFormState,
  type StateSetter,
  incidentBadgeStatus,
} from "./status-pages-shared";

type StatusPageIncidentsPanelProps = {
  components: ApiStatusPageComponentResponse[];
  createForm: IncidentFormState;
  createIncidentError: boolean;
  createIncidentPending: boolean;
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
  onDeleteIncident: () => void;
  onPublishIncident: () => void;
  onResolveIncident: () => void;
  onSelectIncident: (id: string) => void;
  onSubmitCreateIncident: (event: FormEvent) => void;
  onSubmitEditIncident: (event: FormEvent) => void;
  onSubmitIncidentUpdate: (event: FormEvent) => void;
  pageId: string;
  selectedIncident?: ApiStatusPageIncidentResponse;
  selectedIncidentId: string;
  setCreateForm: StateSetter<IncidentFormState>;
  setEditForm: StateSetter<IncidentFormState>;
  setUpdateForm: StateSetter<IncidentUpdateFormState>;
  updateForm: IncidentUpdateFormState;
  updateIncidentError: boolean;
  updateIncidentPending: boolean;
};

export const StatusPageIncidentsPanel = ({
  components,
  createForm,
  createIncidentError,
  createIncidentPending,
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
  onDeleteIncident,
  onPublishIncident,
  onResolveIncident,
  onSelectIncident,
  onSubmitCreateIncident,
  onSubmitEditIncident,
  onSubmitIncidentUpdate,
  pageId,
  selectedIncident,
  selectedIncidentId,
  setCreateForm,
  setEditForm,
  setUpdateForm,
  updateForm,
  updateIncidentError,
  updateIncidentPending,
}: StatusPageIncidentsPanelProps) => (
  <section className="grid gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
    <CreateIncidentForm
      components={components}
      form={createForm}
      internalIncidents={internalIncidents}
      isError={createIncidentError}
      isPending={createIncidentPending}
      onSubmit={onSubmitCreateIncident}
      pageId={pageId}
      setForm={setCreateForm}
      suggestions={createSuggestions}
      suggestionsError={createSuggestionsError}
      suggestionsLoading={createSuggestionsLoading}
    />

    <div className="space-y-4">
      <ConfiguredIncidents
        incidents={incidents}
        onSelectIncident={onSelectIncident}
        selectedIncidentId={selectedIncidentId}
      />
      {selectedIncident && (
        <div className="grid gap-4 xl:grid-cols-2">
          <EditIncidentForm
            components={components}
            deleteIncidentPending={deleteIncidentPending}
            deletePending={deletePending}
            form={editForm}
            isError={updateIncidentError}
            isPending={updateIncidentPending}
            onDelete={onDeleteIncident}
            onPublish={onPublishIncident}
            onResolve={onResolveIncident}
            onSubmit={onSubmitEditIncident}
            setForm={setEditForm}
            suggestions={editSuggestions}
            suggestionsError={editSuggestionsError}
            suggestionsLoading={editSuggestionsLoading}
          />
          <IncidentUpdatesPanel
            createUpdateError={createUpdateError}
            createUpdatePending={createUpdatePending}
            form={updateForm}
            onSubmit={onSubmitIncidentUpdate}
            selectedIncident={selectedIncident}
            setForm={setUpdateForm}
          />
        </div>
      )}
    </div>
  </section>
);

const ConfiguredIncidents = ({
  incidents,
  onSelectIncident,
  selectedIncidentId,
}: {
  incidents: ApiStatusPageIncidentResponse[];
  onSelectIncident: (id: string) => void;
  selectedIncidentId: string;
}) => (
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
          onClick={() => onSelectIncident(incident.id ?? "")}
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
            {incident.internal_incident_id ? ` - linked ${incident.internal_incident_id}` : ""}
          </span>
        </button>
      ))}
    </div>
  </div>
);
