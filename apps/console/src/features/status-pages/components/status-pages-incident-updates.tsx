import { StatusBadge } from "@/components/shared/status-badges";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import type { ApiStatusPageIncidentResponse } from "@/orion-sdk";
import { Plus } from "lucide-react";
import type { FormEvent } from "react";
import { SelectField } from "./status-pages-create-incident-form";
import {
  DateTimeInput,
  Field,
  type IncidentUpdateFormState,
  type StateSetter,
  formatDateTime,
  incidentBadgeStatus,
  incidentStatuses,
} from "./status-pages-shared";

type IncidentUpdatesPanelProps = {
  createUpdateError: boolean;
  createUpdatePending: boolean;
  form: IncidentUpdateFormState;
  onSubmit: (event: FormEvent) => void;
  selectedIncident: ApiStatusPageIncidentResponse;
  setForm: StateSetter<IncidentUpdateFormState>;
};

export const IncidentUpdatesPanel = ({
  createUpdateError,
  createUpdatePending,
  form,
  onSubmit,
  selectedIncident,
  setForm,
}: IncidentUpdatesPanelProps) => (
  <div className="space-y-4">
    <form className="space-y-3" onSubmit={onSubmit}>
      <h3 className="text-sm font-medium">Add Public Update</h3>
      <div className="grid gap-3 sm:grid-cols-2">
        <SelectField
          label="Status"
          options={incidentStatuses}
          value={form.status}
          onChange={(value) => setForm((current) => ({ ...current, status: value }))}
        />
        <Field label="Published at">
          <DateTimeInput
            value={form.publishedAt}
            onChange={(value) => setForm((current) => ({ ...current, publishedAt: value }))}
          />
        </Field>
      </div>
      <Field label="Message">
        <Textarea
          value={form.message}
          onChange={(event) => setForm((current) => ({ ...current, message: event.target.value }))}
          rows={4}
        />
      </Field>
      <Field label="Created by">
        <Input
          value={form.createdBy}
          onChange={(event) =>
            setForm((current) => ({ ...current, createdBy: event.target.value }))
          }
          placeholder="Support"
        />
      </Field>
      <Button disabled={!form.message.trim() || createUpdatePending}>
        <Plus className="size-4" />
        {createUpdatePending ? "Adding..." : "Add update"}
      </Button>
      {createUpdateError && <p className="text-sm">Unable to add public update.</p>}
    </form>

    <div className="space-y-2">
      <h3 className="text-sm font-medium">Public Updates</h3>
      {(selectedIncident.updates ?? []).length === 0 && (
        <div className="text-sm text-neutral-600">No updates yet.</div>
      )}
      {(selectedIncident.updates ?? []).map((update) => (
        <div className="border border-neutral-200 p-3 text-sm" key={update.id}>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <StatusBadge fallback={update.status} value={incidentBadgeStatus(update.status)} />
            <span className="text-neutral-600">
              {formatDateTime(update.published_at ?? update.created_at)}
            </span>
          </div>
          <p className="mt-2 whitespace-pre-wrap">{update.message}</p>
          {update.created_by && <div className="mt-2 text-neutral-600">By {update.created_by}</div>}
        </div>
      ))}
    </div>
  </div>
);
