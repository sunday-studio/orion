import { StatusBadge } from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import type { ApiStatusPageResponse } from "@/orion-sdk";
import { ExternalLink, Globe2, Trash2 } from "lucide-react";
import { publicUrl } from "./status-pages-shared";

type StatusPageHeaderProps = {
  canPublish: boolean;
  deletePagePending: boolean;
  deletePending: boolean;
  onDeletePage: () => void;
  onPublishPage: () => void;
  onUnpublishPage: () => void;
  page: ApiStatusPageResponse;
  publishBlockers: string[];
  publishError: boolean;
  publishPending: boolean;
  publishWarnings: string[];
  unpublishPending: boolean;
};

export const StatusPageHeader = ({
  canPublish,
  deletePagePending,
  deletePending,
  onDeletePage,
  onPublishPage,
  onUnpublishPage,
  page,
  publishBlockers,
  publishError,
  publishPending,
  publishWarnings,
  unpublishPending,
}: StatusPageHeaderProps) => (
  <section className="flex flex-wrap items-start justify-between gap-3">
    <div>
      <div className="flex flex-wrap items-center gap-2">
        <h2 className="text-lg font-semibold">{page.title}</h2>
        <StatusBadge
          fallback={page.visibility}
          value={page.visibility === "public" ? "up" : "unknown"}
        />
      </div>
      <div className="mt-1 flex flex-wrap items-center gap-3 text-sm text-neutral-600">
        <span>{page.slug}</span>
        {page.slug && (
          <a className="inline-flex items-center gap-1" href={publicUrl(page.slug)}>
            <ExternalLink className="size-3.5" />
            Public URL
          </a>
        )}
      </div>
    </div>
    <div className="flex flex-wrap gap-2">
      <Button disabled={publishPending || !canPublish} onClick={onPublishPage}>
        <Globe2 className="size-4" />
        {publishPending ? "Publishing..." : "Publish"}
      </Button>
      <Button
        disabled={unpublishPending || page.visibility !== "public"}
        onClick={onUnpublishPage}
        variant="outline"
      >
        {unpublishPending ? "Unpublishing..." : "Unpublish"}
      </Button>
      <Button disabled={deletePending} onClick={onDeletePage} type="button" variant="outline">
        <Trash2 className="size-4" />
        {deletePagePending ? "Deleting..." : "Delete"}
      </Button>
    </div>
    {publishError && (
      <div className="basis-full text-sm">Unable to publish. Check visible components and mappings.</div>
    )}
    {(publishBlockers.length > 0 || publishWarnings.length > 0) && (
      <div className="basis-full space-y-1 text-sm">
        {publishBlockers.map((blocker) => (
          <div className="text-rose-700" key={blocker}>
            {blocker}
          </div>
        ))}
        {publishWarnings.map((warning) => (
          <div className="text-amber-700" key={warning}>
            {warning}
          </div>
        ))}
      </div>
    )}
  </section>
);

export const StatusPageTabSwitch = ({
  activePageTab,
  onChange,
}: {
  activePageTab: "setup" | "subscribers";
  onChange: (tab: "setup" | "subscribers") => void;
}) => (
  <div className="inline-flex border border-neutral-800">
    <button
      className={`px-3 py-1.5 text-sm ${
        activePageTab === "setup" ? "bg-neutral-800 text-white" : "bg-white"
      }`}
      onClick={() => onChange("setup")}
      type="button"
    >
      Setup
    </button>
    <button
      className={`border-l border-neutral-800 px-3 py-1.5 text-sm ${
        activePageTab === "subscribers" ? "bg-neutral-800 text-white" : "bg-white"
      }`}
      onClick={() => onChange("subscribers")}
      type="button"
    >
      Subscribers
    </button>
  </div>
);
