import { Plus } from "lucide-react";
import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Button,
  Callout,
  EmptyState,
  FieldPanel,
  GridRow,
  GridTable,
  RaisedCard,
  Skeleton,
  StatBox,
  StatusBadge,
} from "@/design-system";
import { NewAppDialog } from "@/features/apps/dialogs/new-app-dialog";
import {
  agoShort,
  appStatusLabel,
  appStatusTone,
  compact,
  gigabytes,
  ratePerSecond,
  storageUnit,
} from "@/features/apps/app-format";
import { useGetLogIngestStatus, useListApps } from "@/orion-sdk";

const COLUMNS = "minmax(0,1.2fr) 120px 110px 110px 80px";

export const AppsTab = () => {
  const navigate = useNavigate();
  const apps = useListApps();
  const ingest = useGetLogIngestStatus();
  const [creating, setCreating] = useState(false);
  const list = apps.data?.apps ?? [];
  const status = ingest.data;
  return (
    <div className="flex flex-col gap-3.5">
      <div className="grid grid-cols-3 gap-3.5">
        <StatBox label="Lines today" value={compact(status?.lines_today)} />
        <StatBox label="Dropped" value={(status?.dropped ?? 0).toLocaleString()} />
        <StatBox
          label="Log storage"
          value={
            <>
              {gigabytes(status?.storage_bytes)}{" "}
              {status?.storage_limit_bytes ? (
                <span className="text-sm font-normal text-faint">
                  of {gigabytes(status.storage_limit_bytes)}{" "}
                  {storageUnit(status.storage_limit_bytes)}
                </span>
              ) : (
                <span className="text-sm font-normal text-faint">
                  {storageUnit(status?.storage_bytes)}
                </span>
              )}
            </>
          }
        />
      </div>
      <FieldPanel title="Apps" aside="Each app has its own keys, retention and rules">
        {apps.isPending ? (
          <Skeleton className="h-32 w-full" />
        ) : apps.isError ? (
          <Callout tone="down">Unable to load apps.</Callout>
        ) : list.length === 0 ? (
          <EmptyState
            className="rounded-xl border-solid bg-card"
            title="No apps yet"
            description="An app is anything that sends logs: a service, a cron job, a script."
          />
        ) : (
          <RaisedCard>
            <GridTable>
              <GridRow columns={COLUMNS} header>
                <span>App</span>
                <span>Environment</span>
                <span>Status</span>
                <span>Last log</span>
                <span>Rate</span>
              </GridRow>
              {list.map((app) => (
                <GridRow
                  key={app.id}
                  columns={COLUMNS}
                  className="cursor-pointer"
                  onClick={() => navigate(`/settings/apps/${app.id}`)}
                >
                  <Link
                    to={`/settings/apps/${app.id}`}
                    className="truncate font-medium outline-none hover:underline focus-visible:underline"
                    onClick={(event) => event.stopPropagation()}
                  >
                    {app.name}
                  </Link>
                  <span className="text-faint">{app.environment || "-"}</span>
                  <span>
                    <StatusBadge status={appStatusTone(app.status)}>
                      {appStatusLabel(app.status)}
                    </StatusBadge>
                  </span>
                  <span className="text-faint">{agoShort(app.last_ingest_at)}</span>
                  <span className="text-faint">
                    {app.status === "never" ? "-" : ratePerSecond(app.rate_per_second)}
                  </span>
                </GridRow>
              ))}
            </GridTable>
          </RaisedCard>
        )}
        <Button size="sm" className="mt-3" onClick={() => setCreating(true)}>
          <Plus className="size-3.5" />
          New app
        </Button>
      </FieldPanel>
      <NewAppDialog open={creating} onOpenChange={setCreating} />
    </div>
  );
};
