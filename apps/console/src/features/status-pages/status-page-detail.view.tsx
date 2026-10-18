import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import {
  Button,
  EmptyState,
  Skeleton,
  StatusBadge,
  Tabs,
  TabsList,
  TabsPanel,
  TabsTab,
  useToast,
} from "@/design-system";
import {
  getGetStatusPageBadgesQueryKey,
  getGetStatusPageQueryKey,
  getGetStatusPageReadinessQueryKey,
  getListStatusPagesQueryKey,
  getPreviewStatusPageQueryKey,
  useGetStatusPage,
  useGetStatusPageReadiness,
  useListStatusPages,
  usePublishStatusPage,
  useUnpublishStatusPage,
} from "@/orion-sdk";
import { PublishDialog, UnpublishDialog } from "./dialogs/publish-dialogs";
import { stateLabel, stateTone } from "./status-format";
import { ComponentsTab } from "./tabs/components-tab";
import { IncidentsTab } from "./tabs/incidents-tab";
import { MaintenanceTab } from "./tabs/maintenance-tab";
import { PreviewTab } from "./tabs/preview-tab";
import { SetupTab } from "./tabs/setup-tab";
import { SubscribersTab } from "./tabs/subscribers-tab";

const TABS = ["setup", "components", "incidents", "maintenance", "subscribers", "preview"] as const;
type Tab = (typeof TABS)[number];

export const StatusPageDetailPage = () => {
  const { pageId = "" } = useParams();
  const [params, setParams] = useSearchParams();
  const tab: Tab = TABS.includes(params.get("tab") as Tab) ? (params.get("tab") as Tab) : "setup";
  const queryClient = useQueryClient();
  const toast = useToast();
  const detail = useGetStatusPage(pageId);
  const readiness = useGetStatusPageReadiness(pageId);
  const list = useListStatusPages();
  const row = list.data?.pages?.find((item) => item.id === pageId);
  const [publishing, setPublishing] = useState(false);
  const [unpublishing, setUnpublishing] = useState(false);
  const [publishError, setPublishError] = useState<string>();

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: getGetStatusPageQueryKey(pageId) });
    void queryClient.invalidateQueries({ queryKey: getGetStatusPageReadinessQueryKey(pageId) });
    void queryClient.invalidateQueries({ queryKey: getPreviewStatusPageQueryKey(pageId) });
    void queryClient.invalidateQueries({ queryKey: getGetStatusPageBadgesQueryKey(pageId) });
    void queryClient.invalidateQueries({ queryKey: getListStatusPagesQueryKey() });
  };
  const publish = usePublishStatusPage({
    mutation: {
      onSuccess: () => {
        refresh();
        setPublishing(false);
        toast.add({ title: "Page published" });
      },
      onError: (failure) => setPublishError((failure as Error).message || "Unable to publish."),
    },
  });
  const unpublish = useUnpublishStatusPage({
    mutation: {
      onSuccess: () => {
        refresh();
        setUnpublishing(false);
        toast.add({ title: "Page unpublished" });
      },
      onError: () => toast.add({ title: "Unable to unpublish the page" }),
    },
  });

  const setTab = (next: string) =>
    setParams(
      (current) => {
        const copy = new URLSearchParams(current);
        if (next === "setup") copy.delete("tab");
        else copy.set("tab", next);
        return copy;
      },
      { replace: true },
    );

  if (detail.isPending) return <Skeleton className="mt-10 h-40 w-full" />;
  const page = detail.data?.page;
  if (detail.isError || !page) {
    return <EmptyState tone="error" title="Unable to load the status page" />;
  }
  const published = row?.state ? row.state !== "draft" : page.visibility === "published";
  const state = row?.state ?? (published ? "published" : "draft");

  return (
    <div className="w-full pb-8">
      <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-2.5 pb-2 pt-7">
        <div className="min-w-0">
          <div className="text-[13.5px] text-quiet">
            <Link to="/status-pages" className="hover:text-ink">
              Status pages
            </Link>
          </div>
          <h1 className="text-[28px] font-medium leading-10 tracking-[-0.025em]">{page.title}</h1>
          <div className="mt-1.5 flex items-center gap-2 text-[13.5px] text-quiet">
            <StatusBadge status={stateTone(state)} shape={state === "draft" ? "hollow" : undefined}>
              {stateLabel(state)}
            </StatusBadge>
            <span>{row?.public_url ?? `/status/${page.slug}`}</span>
          </div>
        </div>
        <div className="flex items-center gap-1.5">
          <Button size="sm" variant="secondary" onClick={() => setTab("preview")}>
            Preview
          </Button>
          {published ? (
            <Button size="sm" variant="secondary" onClick={() => setUnpublishing(true)}>
              Unpublish
            </Button>
          ) : (
            <Button
              size="sm"
              onClick={() => {
                setPublishError(undefined);
                setPublishing(true);
              }}
            >
              Publish
            </Button>
          )}
        </div>
      </div>
      <Tabs value={tab} onValueChange={setTab} className="mt-3 gap-3">
        <TabsList>
          <TabsTab value="setup">Setup</TabsTab>
          <TabsTab value="components">Components</TabsTab>
          <TabsTab value="incidents">Incidents</TabsTab>
          <TabsTab value="maintenance">Maintenance</TabsTab>
          <TabsTab value="subscribers">Subscribers</TabsTab>
          <TabsTab value="preview">Preview</TabsTab>
        </TabsList>
        <TabsPanel value="setup">
          <SetupTab
            key={page.updated_at ?? pageId}
            pageId={pageId}
            page={page}
            readiness={readiness.data}
            onSaved={refresh}
          />
        </TabsPanel>
        <TabsPanel value="components">
          <ComponentsTab pageId={pageId} detail={detail.data} onChanged={refresh} />
        </TabsPanel>
        <TabsPanel value="incidents">
          <IncidentsTab pageId={pageId} detail={detail.data} onChanged={refresh} />
        </TabsPanel>
        <TabsPanel value="maintenance">
          <MaintenanceTab pageId={pageId} components={detail.data?.components ?? []} />
        </TabsPanel>
        <TabsPanel value="subscribers">
          <SubscribersTab pageId={pageId} />
        </TabsPanel>
        <TabsPanel value="preview">
          <PreviewTab
            pageId={pageId}
            published={published}
            url={row?.public_url ?? `/status/${page.slug}`}
          />
        </TabsPanel>
      </Tabs>
      <PublishDialog
        open={publishing}
        title={page.title ?? "this page"}
        readiness={readiness.data}
        pending={publish.isPending}
        error={publishError}
        onOpenChange={setPublishing}
        onPublish={() => publish.mutate({ id: pageId })}
      />
      <UnpublishDialog
        open={unpublishing}
        title={page.title ?? "this page"}
        pending={unpublish.isPending}
        onOpenChange={setUnpublishing}
        onUnpublish={() => unpublish.mutate({ id: pageId })}
      />
    </div>
  );
};
