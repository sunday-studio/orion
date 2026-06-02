import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { ApiStatusPageComponentResponse, ApiStatusPageSectionResponse } from "@/orion-sdk";
import { Link2, Plus, Trash2 } from "lucide-react";
import type { FormEvent } from "react";
import {
  type ComponentFormState,
  Field,
  type MappingFormState,
  type SectionFormState,
  type StateSetter,
  manualStatuses,
} from "./status-pages-shared";

type StatusPageSetupFormsProps = {
  componentForm: ComponentFormState;
  components: ApiStatusPageComponentResponse[];
  createComponentPending: boolean;
  createMappingPending: boolean;
  createSectionPending: boolean;
  deletePending: boolean;
  mappingForm: MappingFormState;
  onRemoveSection: (sectionId?: string, label?: string) => void;
  onSubmitComponent: (event: FormEvent) => void;
  onSubmitMapping: (event: FormEvent) => void;
  onSubmitSection: (event: FormEvent) => void;
  pageId: string;
  resourceOptions: { id: string; label: string }[];
  sectionForm: SectionFormState;
  sections: ApiStatusPageSectionResponse[];
  setComponentForm: StateSetter<ComponentFormState>;
  setMappingForm: StateSetter<MappingFormState>;
  setSectionForm: StateSetter<SectionFormState>;
};

export const StatusPageSetupForms = ({
  componentForm,
  components,
  createComponentPending,
  createMappingPending,
  createSectionPending,
  deletePending,
  mappingForm,
  onRemoveSection,
  onSubmitComponent,
  onSubmitMapping,
  onSubmitSection,
  pageId,
  resourceOptions,
  sectionForm,
  sections,
  setComponentForm,
  setMappingForm,
  setSectionForm,
}: StatusPageSetupFormsProps) => (
  <section className="grid gap-4 xl:grid-cols-3">
    <form className="space-y-3" onSubmit={onSubmitSection}>
      <h3 className="text-sm font-medium">Sections</h3>
      <Field label="Name">
        <Input
          value={sectionForm.name}
          onChange={(event) => setSectionForm({ name: event.target.value })}
          placeholder="API"
        />
      </Field>
      <Button disabled={!pageId || createSectionPending} variant="outline">
        <Plus className="size-4" />
        Add section
      </Button>
      <div className="space-y-2">
        {sections.map((section) => (
          <div
            className="flex items-center justify-between gap-2 border border-neutral-200 px-2 py-1.5 text-sm"
            key={section.id}
          >
            <span className="min-w-0 truncate">{section.name}</span>
            <Button
              disabled={deletePending}
              onClick={() => onRemoveSection(section.id, section.name)}
              size="sm"
              type="button"
              variant="outline"
            >
              <Trash2 className="size-3.5" />
              Remove
            </Button>
          </div>
        ))}
      </div>
    </form>

    <form className="space-y-3" onSubmit={onSubmitComponent}>
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
          {sections.map((section) => (
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
            setComponentForm((current) => ({ ...current, publicName: event.target.value }))
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
            setComponentForm((current) => ({ ...current, manualStatus: event.target.value }))
          }
        >
          {manualStatuses.map((status) => (
            <option key={status.value} value={status.value}>
              {status.label}
            </option>
          ))}
        </select>
      </Field>
      <Button disabled={!componentForm.sectionId || createComponentPending} variant="outline">
        <Plus className="size-4" />
        Add component
      </Button>
    </form>

    <form className="space-y-3" onSubmit={onSubmitMapping}>
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
          {components.map((component) => (
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
          <option value="agent">Server</option>
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
          {resourceOptions.map((resource) => (
            <option key={resource.id} value={resource.id}>
              {resource.label}
            </option>
          ))}
        </select>
      </Field>
      <Button
        disabled={!mappingForm.componentId || !mappingForm.resourceId || createMappingPending}
        variant="outline"
      >
        <Link2 className="size-4" />
        Add mapping
      </Button>
    </form>
  </section>
);
