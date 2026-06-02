import { EmptyState } from "@/components/shared/empty-state";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import type { ApiStatusPageResponse } from "@/orion-sdk";
import { Plus } from "lucide-react";
import type { FormEvent } from "react";
import { Field, type PageFormState, type StateSetter } from "./status-pages-shared";

type StatusPagesSidebarProps = {
  createPageError: boolean;
  createPagePending: boolean;
  onSelectPage: (page: ApiStatusPageResponse) => void;
  onSubmitPage: (event: FormEvent) => void;
  pageForm: PageFormState;
  pages: ApiStatusPageResponse[];
  pagesError: boolean;
  pagesLoading: boolean;
  selectedPageId?: string;
  setPageForm: StateSetter<PageFormState>;
};

export const StatusPagesSidebar = ({
  createPageError,
  createPagePending,
  onSelectPage,
  onSubmitPage,
  pageForm,
  pages,
  pagesError,
  pagesLoading,
  selectedPageId,
  setPageForm,
}: StatusPagesSidebarProps) => (
  <aside className="space-y-4">
    <form className="space-y-3" onSubmit={onSubmitPage}>
      <h2 className="text-sm font-medium">New Page</h2>
      <Field label="Slug">
        <Input
          value={pageForm.slug}
          onChange={(event) => setPageForm((current) => ({ ...current, slug: event.target.value }))}
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
      <Button className="w-full" disabled={createPagePending}>
        <Plus className="size-4" />
        {createPagePending ? "Creating..." : "Create page"}
      </Button>
      {createPageError && <p className="text-sm">Unable to create page.</p>}
    </form>

    <section className="space-y-2">
      <h2 className="text-sm font-medium">Pages</h2>
      {pagesLoading && <div className="text-sm text-neutral-600">Loading...</div>}
      {pagesError && <div className="text-sm">Unable to load status pages.</div>}
      {pages.map((page) => (
        <button
          className={`block w-full border px-3 py-2 text-left text-sm ${
            page.id === selectedPageId
              ? "border-neutral-950 bg-neutral-100"
              : "border-neutral-200 hover:bg-neutral-50"
          }`}
          key={page.id}
          onClick={() => onSelectPage(page)}
          type="button"
        >
          <span className="block font-medium">{page.title}</span>
          <span className="text-neutral-600">{page.slug}</span>
        </button>
      ))}
      {!pagesLoading && pages.length === 0 && (
        <EmptyState title="No status pages" description="Create a draft page to start." />
      )}
    </section>
  </aside>
);
