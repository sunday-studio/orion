import { useQueryClient } from "@tanstack/react-query";
import { parseAsString, useQueryState } from "nuqs";
import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Button,
  ConfirmDialog,
  EmptyState,
  Skeleton,
  StatusBadge,
  Tabs,
  TabsList,
  TabsTab,
  useToast,
} from "@/design-system";
import { getGetAppQueryKey, getListAppsQueryKey, useDeleteApp, useGetApp } from "@/orion-sdk";
import { agoShort, appStatusLabel, appStatusTone } from "./app-format";
import { DisplayTab } from "./tabs/display-tab";
import { IngestTab } from "./tabs/ingest-tab";
import { RedactionTab } from "./tabs/redaction-tab";
import { RetentionTab } from "./tabs/retention-tab";

const TABS = [
  { value: "ingest", label: "Ingest" },
  { value: "retention", label: "Retention" },
  { value: "display", label: "Display" },
  { value: "redaction", label: "Redaction" },
] as const;

export const AppDetailPage = () => {
  const { appId = "" } = useParams();
  const navigate = useNavigate();
  const toast = useToast();
  const queryClient = useQueryClient();
  const [tab, setTab] = useQueryState("tab", parseAsString.withDefault("ingest"));
  const [deleting, setDeleting] = useState(false);
  const query = useGetApp(appId);
  const app = query.data?.app;
  const remove = useDeleteApp({
    mutation: {
      onSuccess: () => {
        void queryClient.invalidateQueries({ queryKey: getListAppsQueryKey() });
        toast.add({ title: "App deleted" });
        navigate("/settings?tab=apps");
      },
      onError: () => toast.add({ title: "Unable to delete the app" }),
    },
  });
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: getGetAppQueryKey(appId) });
    void queryClient.invalidateQueries({ queryKey: getListAppsQueryKey() });
  };

  if (query.isPending) return <Skeleton className="mt-10 h-40 w-full" />;
  if (query.isError || !app) {
    return (
      <EmptyState
        tone="error"
        title="Unable to load the app"
        action={
          <Link to="/settings?tab=apps">
            <Button variant="secondary">Back to apps</Button>
          </Link>
        }
      />
    );
  }
  const active = TABS.some((item) => item.value === tab) ? tab : "ingest";
  return (
    <div className="w-full pb-10">
      <div className="flex flex-wrap items-start justify-between gap-3 pb-2 pt-7">
        <div className="min-w-0">
          <h1 className="text-[28px] font-medium leading-10 tracking-[-0.025em]">{app.name}</h1>
          <div className="mt-1.5 flex items-center gap-2 text-[13.5px] text-quiet">
            <StatusBadge status={appStatusTone(app.status)}>
              {appStatusLabel(app.status)}
            </StatusBadge>
            <span>
              {app.environment || "no environment"} ·{" "}
              {app.last_ingest_at ? `last log ${agoShort(app.last_ingest_at)}` : "no logs yet"}
            </span>
          </div>
        </div>
        <div className="flex gap-1.5">
          <Link to={`/logs?source=${app.id}`}>
            <Button size="sm" variant="secondary">
              Open in Logs
            </Button>
          </Link>
          <Button size="sm" variant="secondary" onClick={() => setDeleting(true)}>
            Delete app
          </Button>
        </div>
      </div>
      <div className="pb-4 pt-3">
        <Tabs value={active} onValueChange={(next) => void setTab(next)}>
          <TabsList>
            {TABS.map((item) => (
              <TabsTab key={item.value} value={item.value}>
                {item.label}
              </TabsTab>
            ))}
          </TabsList>
        </Tabs>
      </div>
      {active === "ingest" && <IngestTab appId={appId} appName={app.name ?? ""} />}
      {active === "retention" && <RetentionTab key={app.id} app={app} onSaved={refresh} />}
      {active === "display" && <DisplayTab key={app.id} app={app} onSaved={refresh} />}
      {active === "redaction" && <RedactionTab key={app.id} app={app} onSaved={refresh} />}
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={`Delete ${app.name}`}
        description="The app, its ingest keys and all of its logs are deleted. This cannot be undone."
        confirmLabel="Delete app"
        onConfirm={() => remove.mutate({ id: appId })}
      />
    </div>
  );
};
