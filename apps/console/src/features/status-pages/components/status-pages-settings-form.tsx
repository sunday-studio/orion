import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { CheckCircle2 } from "lucide-react";
import type { FormEvent } from "react";
import {
  Field,
  type PageSettingsFormState,
  type StateSetter,
  componentDensityOptions,
  headerStyleOptions,
  incidentVisibilities,
  themeModeOptions,
} from "./status-pages-shared";

type StatusPageSettingsFormProps = {
  form: PageSettingsFormState;
  isError: boolean;
  isPending: boolean;
  onSubmit: (event: FormEvent) => void;
  pageId: string;
  setForm: StateSetter<PageSettingsFormState>;
};

export const StatusPageSettingsForm = ({
  form,
  isError,
  isPending,
  onSubmit,
  pageId,
  setForm,
}: StatusPageSettingsFormProps) => (
  <form className="space-y-4" onSubmit={onSubmit}>
    <div className="flex flex-wrap items-center justify-between gap-3">
      <h3 className="text-sm font-medium">Page Settings</h3>
      <Button disabled={!pageId || isPending} variant="outline">
        <CheckCircle2 className="size-4" />
        {isPending ? "Saving..." : "Save settings"}
      </Button>
    </div>
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_320px]">
      <div className="space-y-3">
        <Field label="Public description">
          <Textarea
            value={form.description}
            onChange={(event) =>
              setForm((current) => ({ ...current, description: event.target.value }))
            }
            rows={4}
          />
        </Field>
        <Field label="Default incident visibility">
          <select
            className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
            value={form.defaultIncidentVisibility}
            onChange={(event) =>
              setForm((current) => ({
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
            value={form.canonicalUrl}
            onChange={(event) =>
              setForm((current) => ({ ...current, canonicalUrl: event.target.value }))
            }
          />
        </Field>
      </div>

      <div className="space-y-3">
        <Field label="SEO title">
          <Input
            value={form.seoTitle}
            onChange={(event) =>
              setForm((current) => ({ ...current, seoTitle: event.target.value }))
            }
          />
        </Field>
        <Field label="SEO description">
          <Textarea
            value={form.seoDescription}
            onChange={(event) =>
              setForm((current) => ({ ...current, seoDescription: event.target.value }))
            }
            rows={3}
          />
        </Field>
        <Field label="Open Graph image URL">
          <Input
            placeholder="https://status.example.com/og.png"
            type="url"
            value={form.openGraphImageUrl}
            onChange={(event) =>
              setForm((current) => ({ ...current, openGraphImageUrl: event.target.value }))
            }
          />
        </Field>
      </div>

      <div className="space-y-3">
        <div className="grid gap-3 sm:grid-cols-[96px_minmax(0,1fr)] xl:grid-cols-1">
          <Field label="Accent color">
            <Input
              type="color"
              value={form.accentColor}
              onChange={(event) =>
                setForm((current) => ({ ...current, accentColor: event.target.value }))
              }
            />
          </Field>
          <Field label="Logo URL">
            <Input
              placeholder="https://status.example.com/logo.svg"
              type="url"
              value={form.logoUrl}
              onChange={(event) =>
                setForm((current) => ({ ...current, logoUrl: event.target.value }))
              }
            />
          </Field>
        </div>
        <Field label="Logo alt text">
          <Input
            value={form.logoAlt}
            onChange={(event) =>
              setForm((current) => ({ ...current, logoAlt: event.target.value }))
            }
          />
        </Field>
        <div className="grid gap-3 sm:grid-cols-2">
          <SelectField
            label="Header style"
            options={headerStyleOptions}
            value={form.headerStyle}
            onChange={(value) => setForm((current) => ({ ...current, headerStyle: value }))}
          />
          <SelectField
            label="Theme mode"
            options={themeModeOptions}
            value={form.themeMode}
            onChange={(value) => setForm((current) => ({ ...current, themeMode: value }))}
          />
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <SelectField
            label="Component density"
            options={componentDensityOptions}
            value={form.componentDensity}
            onChange={(value) => setForm((current) => ({ ...current, componentDensity: value }))}
          />
        </div>
        <div className="grid gap-2 text-sm sm:grid-cols-2 xl:grid-cols-1">
          <label className="flex items-center gap-2">
            <input
              checked={form.showUptimeSummary}
              onChange={(event) =>
                setForm((current) => ({ ...current, showUptimeSummary: event.target.checked }))
              }
              type="checkbox"
            />
            <span>Show uptime summary</span>
          </label>
          <label className="flex items-center gap-2">
            <input
              checked={form.showIncidentHistory}
              onChange={(event) =>
                setForm((current) => ({
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
    {isError && <p className="text-sm">Unable to save page settings.</p>}
  </form>
);

const SelectField = ({
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
