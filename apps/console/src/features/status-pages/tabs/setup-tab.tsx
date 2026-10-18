import { useState } from "react";
import {
  Button,
  ColorField,
  Dot,
  FieldPanel,
  Input,
  RaisedCard,
  Segmented,
  Textarea,
  useToast,
} from "@/design-system";
import {
  type ApiStatusPageReadinessResponse,
  type ApiStatusPageResponse,
  useGetStatusPageBadges,
  useUpdateStatusPage,
} from "@/orion-sdk";

import { DomainPanel } from "./domain-panel";

const label = "mb-1.5 block text-[12.5px] font-medium text-mid";

export const SetupTab = ({
  pageId,
  page,
  readiness,
  onSaved,
}: {
  pageId: string;
  page: ApiStatusPageResponse;
  readiness?: ApiStatusPageReadinessResponse;
  onSaved: () => void;
}) => {
  const toast = useToast();
  const theme = page.theme_settings as { accent_color?: string; density?: string } | undefined;
  const [title, setTitle] = useState(page.title ?? "");
  const [description, setDescription] = useState(page.description ?? "");
  const [slug, setSlug] = useState(page.slug ?? "");
  const [accent, setAccent] = useState(theme?.accent_color ?? "");
  const [density, setDensity] = useState(theme?.density === "compact" ? "compact" : "standard");
  const dirty =
    title !== (page.title ?? "") ||
    description !== (page.description ?? "") ||
    slug !== (page.slug ?? "") ||
    accent !== (theme?.accent_color ?? "") ||
    density !== (theme?.density === "compact" ? "compact" : "standard");
  const badges = useGetStatusPageBadges(pageId);
  const save = useUpdateStatusPage({
    mutation: {
      onSuccess: () => {
        onSaved();
        toast.add({ title: "Changes saved" });
      },
      onError: (failure) => toast.add({ title: (failure as Error).message || "Unable to save" }),
    },
  });
  const copy = async (text: string | undefined, name: string) => {
    if (!text) return;
    await navigator.clipboard.writeText(text);
    toast.add({ title: `${name} copied` });
  };

  return (
    <div className="flex flex-col gap-4">
      {readiness && (
        <FieldPanel
          title="Ready to publish"
          aside={`${readiness.passed ?? 0} of ${readiness.total ?? 0}`}
        >
          <RaisedCard>
            {(readiness.checks ?? []).map((check) => (
              <div
                key={check.key}
                className="grid grid-cols-[12px_minmax(0,1fr)_auto] items-center gap-3 border-t border-line-soft px-3.5 py-2.5 text-[13px] first:border-t-0"
              >
                <Dot color={check.ok ? "#2a9f69" : check.blocking ? "#e5484d" : "#f59e0b"} />
                <b className="font-medium">{check.label}</b>
                <span className="text-xs text-faint">{check.ok ? "" : check.detail}</span>
              </div>
            ))}
          </RaisedCard>
        </FieldPanel>
      )}
      <FieldPanel title="Page">
        <RaisedCard className="grid gap-3.5 p-4">
          <label>
            <span className={label}>Title</span>
            <Input value={title} onChange={(event) => setTitle(event.target.value)} />
          </label>
          <label>
            <span className={label}>Public description</span>
            <Textarea
              rows={3}
              value={description}
              onChange={(event) => setDescription(event.target.value)}
            />
          </label>
          <label>
            <span className={label}>Address</span>
            <Input value={slug} onChange={(event) => setSlug(event.target.value)} />
            {page.visibility === "published" && slug !== page.slug && (
              <small className="mt-1 block text-xs text-faint">
                This page is published. Changing the address breaks existing links.
              </small>
            )}
          </label>
        </RaisedCard>
      </FieldPanel>
      <FieldPanel title="Branding">
        <RaisedCard className="grid gap-3.5 p-4">
          <div className="grid gap-1.5">
            <span className={label}>Accent color</span>
            <ColorField aria-label="Accent color" value={accent} onChange={setAccent} />
          </div>
          <div className="grid gap-1.5">
            <span className={label}>Layout</span>
            <Segmented
              aria-label="Layout"
              surface="card"
              options={[
                { value: "standard", label: "Standard" },
                { value: "compact", label: "Compact" },
              ]}
              value={density}
              onValueChange={setDensity}
            />
            <small className="text-xs text-quiet">
              Compact drops the summary cards and tightens spacing on the public page. Add
              ?compact=1 to any public page URL to preview it.
            </small>
          </div>
        </RaisedCard>
      </FieldPanel>
      <DomainPanel pageId={pageId} domain={page.custom_domain} onChanged={onSaved} />
      <div className="flex justify-end">
        <Button
          disabled={save.isPending || !title.trim() || !dirty}
          onClick={() =>
            save.mutate({
              id: pageId,
              data: {
                title: title.trim(),
                description,
                slug: slug.trim(),
                theme_settings: { ...page.theme_settings, accent_color: accent.trim(), density },
              },
            })
          }
        >
          Save changes
        </Button>
      </div>
      {badges.data?.page && (
        <FieldPanel title="Badge" aside={badges.data.note}>
          <RaisedCard className="flex items-center justify-between gap-3 p-4">
            <code className="min-w-0 truncate font-mono text-xs text-mid">
              {badges.data.page.svg_url}
            </code>
            <span className="flex gap-1.5">
              <Button
                size="sm"
                variant="secondary"
                onClick={() => void copy(badges.data?.page?.markdown, "Markdown")}
              >
                Copy Markdown
              </Button>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => void copy(badges.data?.page?.html, "HTML")}
              >
                Copy HTML
              </Button>
            </span>
          </RaisedCard>
        </FieldPanel>
      )}
    </div>
  );
};
