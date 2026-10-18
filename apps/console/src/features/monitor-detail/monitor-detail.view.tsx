import { useParams, useSearchParams } from "react-router-dom";
import { EmptyState, Skeleton, Tabs, TabsList, TabsPanel, TabsTab } from "@/design-system";
import { useNow } from "@/features/overview/overview-time";
import { useGetMonitor } from "@/orion-sdk";
import { MonitorTestResultDialog } from "@/features/monitors/create/monitor-test-result-dialog";
import { MonitorHeader } from "./detail/monitor-header";
import { MonitorHeartbeat } from "./detail/monitor-heartbeat";
import { MonitorHistory } from "./detail/monitor-history";
import { MonitorOverview } from "./detail/monitor-overview";
import { MonitorPaused } from "./detail/monitor-paused";
import { useMonitorActions } from "./detail/use-monitor-actions";

export const MonitorDetailPage = () => {
  const { monitorId = "" } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const now = useNow(1000);
  const query = useGetMonitor(monitorId);
  const monitor = query.data?.monitor;
  const summary = query.data?.summary;
  const actions = useMonitorActions(monitorId, monitor?.name ?? "Monitor");
  const tab = searchParams.get("tab") === "history" ? "history" : "overview";

  if (query.isPending) return <Skeleton className="mx-auto mt-10 h-40 w-full max-w-[712px]" />;
  if (query.isError || !monitor) {
    return (
      <EmptyState
        tone="error"
        title="Unable to load monitor"
        description="It may have been deleted, or Core is unreachable."
      />
    );
  }

  const paused = Boolean(summary?.paused ?? monitor.paused);
  const heartbeat = summary?.kind === "heartbeat" || monitor.kind === "heartbeat";
  const narrow = paused || heartbeat;
  const setTab = (next: string) =>
    setSearchParams(
      (params) => {
        const copy = new URLSearchParams(params);
        if (next === "history") copy.set("tab", "history");
        else copy.delete("tab");
        return copy;
      },
      { replace: true },
    );

  return (
    <div
      className={
        narrow ? "mx-auto w-full max-w-[712px] pb-8" : "mx-auto w-full max-w-[1000px] pb-8"
      }
    >
      <MonitorHeader
        monitor={monitor}
        summary={summary}
        paused={paused}
        pending={actions.pending}
        testing={actions.testing}
        breadcrumb={!narrow}
        core={monitor.owner_kind === "core" || monitor.source === "core"}
        heartbeat={heartbeat}
        onTest={actions.test}
        onPause={actions.pause}
        onResume={actions.resume}
      />
      <MonitorTestResultDialog
        result={actions.testResult}
        running={actions.testing}
        error={actions.testError}
        onRunAgain={actions.test}
        onClose={actions.clearTestResult}
      />
      <Tabs value={tab} onValueChange={setTab} className="mt-3 gap-3">
        <TabsList>
          <TabsTab value="overview">{heartbeat ? "Setup" : "Overview"}</TabsTab>
          <TabsTab value="history">Check history</TabsTab>
        </TabsList>
        <TabsPanel value="overview">
          {paused ? (
            <MonitorPaused
              summary={summary}
              pending={actions.pending}
              core={monitor.owner_kind === "core" || monitor.source === "core"}
              onResume={actions.resume}
            />
          ) : heartbeat ? (
            <MonitorHeartbeat monitorId={monitorId} summary={summary} now={now} />
          ) : (
            <MonitorOverview monitorId={monitorId} summary={summary} now={now} />
          )}
        </TabsPanel>
        <TabsPanel value="history">
          <MonitorHistory monitorId={monitorId} monitorName={monitor.name ?? "Monitor"} />
        </TabsPanel>
      </Tabs>
    </div>
  );
};
