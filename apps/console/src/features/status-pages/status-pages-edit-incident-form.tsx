import { StatusBadge } from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import type {
  ApiStatusPageComponentResponse,
  ApiStatusPageIncidentComponentSuggestionResponse,
} from "@/orion-sdk";
import { CheckCircle2, Globe2, Trash2 } from "lucide-react";
import type { FormEvent } from "react";
import { AffectedComponents, SelectField } from "./status-pages-create-incident-form";
import { IncidentSuggestions } from "./status-pages-incident-suggestions";
import {
  applySuggestedIncidentComponents,
  DateTimeInput,
  Field,
  incidentSeverities,
  incidentStatuses,
  incidentVisibilities,
  type IncidentFormState,
  type StateSetter,
} from "./status-pages-shared";

type EditIncidentFormProps = {
  components: ApiStatusPageComponentResponse[];
  deleteIncidentPending: boolean;
  deletePending: boolean;
  form: IncidentFormState;
  isError: boolean;
  isPending: boolean;
  onDelete: () => void;
  onPublish: () => void;
  onResolve: () => void;
  onSubmit: (event: FormEvent) => void;
  setForm: StateSetter<IncidentFormState>;
  suggestions: ApiStatusPageIncidentComponentSuggestionResponse[];
  suggestionsError: boolean;
  suggestionsLoading: boolean;
};

export const EditIncidentForm = ({
  components,
  deleteIncidentPending,
  deletePending,
  form,
  isError,
  isPending,
  onDelete,
  onPublish,
  onResolve,
  onSubmit,
  setForm,
  suggestions,
  suggestionsError,
  suggestionsLoading,
}: EditIncidentFormProps) => (
  <form className="space-y-3" onSubmit={onSubmit}>
    <div className="flex items-center justify-between gap-2">
      <h3 className="text-sm font-medium">Edit Public Incident</h3>
      <StatusBadge fallback={form.visibility} value={form.visibility === "published" ? "up" : "unknown"} />
    </div>
    <Field label="Internal incident ID">
      <Input
        value={form.internalIncidentId}
        onChange={(event) =>
          setForm((current) => ({ ...current, internalIncidentId: event.target.value }))
        }
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
      />
    </Field>
    <div className="grid gap-3 sm:grid-cols-3">
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
      <SelectField
        label="Visibility"
        options={incidentVisibilities}
        value={form.visibility}
        onChange={(value) => setForm((current) => ({ ...current, visibility: value }))}
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
      <Field label="Published at">
        <DateTimeInput
          value={form.publishedAt}
          onChange={(value) => setForm((current) => ({ ...current, publishedAt: value }))}
        />
      </Field>
      <Field label="Resolved at">
        <DateTimeInput
          value={form.resolvedAt}
          onChange={(value) => setForm((current) => ({ ...current, resolvedAt: value }))}
        />
      </Field>
    </div>
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
    <AffectedComponents columns components={components} form={form} setForm={setForm} />
    <div className="flex flex-wrap gap-2">
      <Button disabled={!form.title.trim() || isPending} variant="outline">
        {isPending ? "Saving..." : "Save incident"}
      </Button>
      <Button disabled={isPending} onClick={onPublish} type="button" variant="outline">
        <Globe2 className="size-4" />
        Publish
      </Button>
      <Button disabled={isPending} onClick={onResolve} type="button" variant="outline">
        <CheckCircle2 className="size-4" />
        Resolve
      </Button>
      <Button disabled={deletePending} onClick={onDelete} type="button" variant="outline">
        <Trash2 className="size-4" />
        {deleteIncidentPending ? "Deleting..." : "Delete"}
      </Button>
    </div>
    {isError && <p className="text-sm">Unable to update public incident.</p>}
  </form>
);
