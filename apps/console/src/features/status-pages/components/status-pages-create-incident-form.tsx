import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import type {
  ApiIncidentResponse,
  ApiStatusPageComponentResponse,
  ApiStatusPageIncidentComponentSuggestionResponse,
} from "@/orion-sdk";
import { Plus } from "lucide-react";
import type { FormEvent } from "react";
import { IncidentSuggestions } from "./status-pages-incident-suggestions";
import {
  applySuggestedIncidentComponents,
  DateTimeInput,
  Field,
  incidentOptionLabel,
  incidentSeverities,
  incidentStatuses,
  toggleIncidentComponent,
  type IncidentFormState,
  type StateSetter,
} from "./status-pages-shared";

type CreateIncidentFormProps = {
  components: ApiStatusPageComponentResponse[];
  form: IncidentFormState;
  internalIncidents: ApiIncidentResponse[];
  isError: boolean;
  isPending: boolean;
  onSubmit: (event: FormEvent) => void;
  pageId: string;
  setForm: StateSetter<IncidentFormState>;
  suggestions: ApiStatusPageIncidentComponentSuggestionResponse[];
  suggestionsError: boolean;
  suggestionsLoading: boolean;
};

export const CreateIncidentForm = ({
  components,
  form,
  internalIncidents,
  isError,
  isPending,
  onSubmit,
  pageId,
  setForm,
  suggestions,
  suggestionsError,
  suggestionsLoading,
}: CreateIncidentFormProps) => (
  <form className="space-y-3" onSubmit={onSubmit}>
    <h3 className="text-sm font-medium">New Public Incident</h3>
    <Field label="Internal incident link">
      <select
        className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
        value={form.internalIncidentId}
        onChange={(event) =>
          setForm((current) => ({ ...current, internalIncidentId: event.target.value }))
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
        value={form.internalIncidentId}
        onChange={(event) =>
          setForm((current) => ({ ...current, internalIncidentId: event.target.value }))
        }
        placeholder="incident_..."
      />
    </Field>
    <IncidentSuggestions
      internalIncidentId={form.internalIncidentId.trim()}
      isError={suggestionsError}
      isLoading={suggestionsLoading}
      onApply={() => applySuggestedIncidentComponents(form, setForm, suggestions)}
      suggestions={suggestions}
    />
    <Field label="Public title">
      <Input
        value={form.title}
        onChange={(event) => setForm((current) => ({ ...current, title: event.target.value }))}
        placeholder="API latency elevated"
      />
    </Field>
    <div className="grid gap-3 sm:grid-cols-2">
      <SelectField
        label="Public status"
        options={incidentStatuses}
        value={form.publicStatus}
        onChange={(value) => setForm((current) => ({ ...current, publicStatus: value }))}
      />
      <SelectField
        label="Severity"
        options={incidentSeverities}
        value={form.severity}
        onChange={(value) => setForm((current) => ({ ...current, severity: value }))}
      />
    </div>
    <Field label="Public impact">
      <Textarea
        value={form.impactSummary}
        onChange={(event) =>
          setForm((current) => ({ ...current, impactSummary: event.target.value }))
        }
        rows={3}
      />
    </Field>
    <div className="grid gap-3 sm:grid-cols-2">
      <Field label="Scheduled start">
        <DateTimeInput
          value={form.scheduledStartAt}
          onChange={(value) => setForm((current) => ({ ...current, scheduledStartAt: value }))}
        />
      </Field>
      <Field label="Scheduled end">
        <DateTimeInput
          value={form.scheduledEndAt}
          onChange={(value) => setForm((current) => ({ ...current, scheduledEndAt: value }))}
        />
      </Field>
    </div>
    <AffectedComponents components={components} form={form} setForm={setForm} />
    <Button disabled={!pageId || !form.title.trim() || isPending} variant="outline">
      <Plus className="size-4" />
      {isPending ? "Creating..." : "Create draft incident"}
    </Button>
    {isError && <p className="text-sm">Unable to create public incident.</p>}
  </form>
);

export const AffectedComponents = ({
  columns = false,
  components,
  form,
  setForm,
}: {
  columns?: boolean;
  components: ApiStatusPageComponentResponse[];
  form: IncidentFormState;
  setForm: StateSetter<IncidentFormState>;
}) => (
  <div className="space-y-2">
    <div className="text-sm font-medium">Affected components</div>
    <div className={columns ? "grid gap-1 sm:grid-cols-2" : "space-y-1"}>
      {components.map((component) => (
        <label className="flex items-center gap-2 text-sm" key={component.id}>
          <input
            checked={form.affectedComponentIds.includes(component.id ?? "")}
            onChange={() => component.id && toggleIncidentComponent(form, setForm, component.id)}
            type="checkbox"
          />
          <span>{component.public_name}</span>
        </label>
      ))}
      {components.length === 0 && (
        <div className="text-sm text-neutral-600">No components configured.</div>
      )}
    </div>
  </div>
);

export const SelectField = ({
  label,
  onChange,
  options,
  value,
}: {
  label: string;
  onChange: (value: string) => void;
  options: { label: string; value: string }[];
  value: string;
}) => (
  <Field label={label}>
    <select
      className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
      value={value}
      onChange={(event) => onChange(event.target.value)}
    >
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  </Field>
);
