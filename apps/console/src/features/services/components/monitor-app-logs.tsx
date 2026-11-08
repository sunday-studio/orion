import { useQueries } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { Box, BoxHeader, GridRow, GridTable, StatBox, StatusBadge } from "@/design-system";
import { levelLabel, levelTone } from "@/features/logs/log-model";
import { getSearchLogsQueryOptions, useGetService, useGetServiceLogCounts } from "@/orion-sdk";
import { TIME_FORMAT, formatDate } from "@/utils/date";
import { mergeLogs, openInLogsHref } from "../logs-model";
import { serviceHref } from "../services.domain";
import { totalErrors } from "../timeline-model";

/** "App errors, last hour" for the apps behind a monitor's service. */
export const MonitorAppErrorsStat = ({ serviceId }: { serviceId: string }) => {
  const service = useGetService(serviceId);
  const counts = useGetServiceLogCounts(serviceId, { from: "-1h", step_seconds: 300 });
  const apps = service.data?.service?.apps ?? [];
  return (
    <StatBox
      label="App errors, last hour"
      value={
        apps.length > 0 && counts.data
          ? totalErrors(counts.data.buckets ?? []).toLocaleString()
          : "-"
      }
    />
  );
};

const COLUMNS = "90px 80px 150px minmax(0,1fr)";

/** The latest warnings and errors from the service's apps, so a failing check has its logs beside it. */
export const MonitorRecentAppLogs = ({ serviceId }: { serviceId: string }) => {
  const service = useGetService(serviceId);
  const apps = service.data?.service?.apps ?? [];
  const results = useQueries({
    queries: apps.map((app) =>
      getSearchLogsQueryOptions({
        app_id: app.id ?? "",
        from: "-1h",
        severity: ">=warn",
        limit: 20,
      }),
    ),
  });
  if (apps.length === 0) return null;
  const entries = mergeLogs(results.map((result) => result.data?.entries ?? [])).slice(0, 5);
  const name = service.data?.service?.name;
  return (
    <Box>
      <BoxHeader
        title="Recent app logs"
        aside={
          <Link
            to={openInLogsHref(
              apps.map((app) => app.id ?? ""),
              "1h",
            )}
            className="text-[12.5px] underline"
          >
            Open in Logs
          </Link>
        }
      />
      {results.some((result) => result.isPending) ? (
        <p className="px-3.5 py-3 text-[13px] text-quiet">Loading logs…</p>
      ) : entries.length === 0 ? (
        <p className="px-3.5 py-3 text-[13px] text-quiet">
          No warnings or errors from{" "}
          {name ? (
            <Link to={serviceHref(serviceId)} className="underline">
              {name}
            </Link>
          ) : (
            "its apps"
          )}{" "}
          in the last hour.
        </p>
      ) : (
        <GridTable>
          <GridRow columns={COLUMNS} header>
            <span>Time</span>
            <span>Level</span>
            <span>App</span>
            <span>Message</span>
          </GridRow>
          {entries.map((entry, index) => (
            <GridRow key={entry.id ?? index} columns={COLUMNS}>
              <span className="font-mono text-xs text-quiet">
                {formatDate(entry.timestamp, `${TIME_FORMAT}:ss`)}
              </span>
              <StatusBadge status={levelTone(entry.severity)}>
                {levelLabel(entry.severity)}
              </StatusBadge>
              <span className="truncate">{entry.app_name ?? "-"}</span>
              <span className="truncate">{entry.event ?? entry.body}</span>
            </GridRow>
          ))}
        </GridTable>
      )}
    </Box>
  );
};
