import { DataTable } from "@/components/data-table";
import { ListPagination } from "@/components/list-pagination";
import { TabCount, Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { HeartbeatSetupPanel } from "@/features/monitors/components/heartbeat-setup-panel";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { cn } from "@/lib/utils";
import type {
  ApiCoreMonitorConfigResponse,
  ApiIncidentResponse,
  ApiMonitorReportResponse,
  ApiMonitorResponse,
} from "@/orion-sdk";
import {
  DetailItem,
  HISTORY_LIMIT,
  coreConfigArrayCount,
  coreConfigValue,
  formatJSON,
  historyColumns,
  incidentColumns,
  type MonitorDetailTab,
} from "./monitor-detail-shared";

type MonitorDetailOperationalDataProps = {
  activeTab: MonitorDetailTab;
  coreConfig?: ApiCoreMonitorConfigResponse;
  coreConfigError: unknown;
  coreConfigLoading: boolean;
  handleTabChange: (tab: string) => void;
  highlightedIncidentId: string;
  historyError: unknown;
  historyLoading: boolean;
  historyOffset: number;
  incidentsError: unknown;
  incidentsLoading: boolean;
  isCoreMonitor: boolean;
  monitor: ApiMonitorResponse;
  onHistoryOffsetChange: (offset: number) => void;
  onReportClick: (report: ApiMonitorReportResponse) => void;
  relatedIncidents: ApiIncidentResponse[];
  reportCount: number;
  reports: ApiMonitorReportResponse[];
};

export const MonitorDetailOperationalData = ({
  activeTab,
  coreConfig,
  coreConfigError,
  coreConfigLoading,
  handleTabChange,
  highlightedIncidentId,
  historyError,
  historyLoading,
  historyOffset,
  incidentsError,
  incidentsLoading,
  isCoreMonitor,
  monitor,
  onHistoryOffsetChange,
  onReportClick,
  relatedIncidents,
  reportCount,
  reports,
}: MonitorDetailOperationalDataProps) => (
  <section className="space-y-4">
    <h2 className="text-sm font-medium">Operational Data</h2>
    <Tabs value={activeTab} onValueChange={handleTabChange} className="space-y-3">
      <TabsList>
        <TabsTrigger value="history">
          Check history <TabCount>{reportCount}</TabCount>
        </TabsTrigger>
        <TabsTrigger value="incidents">
          Incidents <TabCount>{relatedIncidents.length}</TabCount>
        </TabsTrigger>
        <TabsTrigger value="config">Configuration</TabsTrigger>
      </TabsList>
      <TabsContent value="history">
        <div className="space-y-3">
          {Boolean(historyError) && <div className="text-sm">Unable to load check history.</div>}
          {!historyError && (
            <DataTable
              columns={historyColumns}
              data={reports}
              emptyMessage="No check history recorded."
              getRowId={(report, index) =>
                report.id ?? `${report.monitor_id ?? "monitor"}-${index}`
              }
              isLoading={historyLoading}
              loadingMessage="Loading check history..."
              onRowClick={onReportClick}
            />
          )}
          {reportCount > 0 && (
            <ListPagination
              count={reportCount}
              limit={HISTORY_LIMIT}
              offset={historyOffset}
              onOffsetChange={onHistoryOffsetChange}
            />
          )}
        </div>
      </TabsContent>
      <TabsContent value="incidents">
        {Boolean(incidentsError) && (
          <div className="text-sm">Unable to load related incidents.</div>
        )}
        {!incidentsError && (
          <DataTable
            columns={incidentColumns}
            data={relatedIncidents}
            emptyMessage="No related incidents recorded."
            getRowId={(incident, index) => incident.id ?? `incident-${index}`}
            isLoading={incidentsLoading}
            loadingMessage="Loading related incidents..."
            rowClassName={(row) => cn(row.original.id === highlightedIncidentId && "bg-amber-50")}
          />
        )}
      </TabsContent>
      <TabsContent value="config">
        <MonitorConfigPanel
          config={coreConfig}
          error={coreConfigError}
          isCoreMonitor={isCoreMonitor}
          isLoading={coreConfigLoading}
          monitor={monitor}
        />
      </TabsContent>
    </Tabs>
  </section>
);

const MonitorConfigPanel = ({
  config,
  error,
  isCoreMonitor,
  isLoading,
  monitor,
}: {
  config?: ApiCoreMonitorConfigResponse;
  error: unknown;
  isCoreMonitor: boolean;
  isLoading: boolean;
  monitor: ApiMonitorResponse;
}) => (
  <div className="space-y-3">
    {!isCoreMonitor && (
      <div className="grid gap-3 sm:grid-cols-3">
        <DetailItem label="type" value={monitor.type ?? "unknown"} />
        <DetailItem label="interval" value={`${monitor.reporting_interval_seconds ?? 0}s`} />
        <DetailItem label="lifecycle" value={monitor.lifecycle ?? "unknown"} />
        <DetailItem label="owner" value="Server configuration" />
      </div>
    )}
    {isCoreMonitor && isLoading && (
      <div className="text-sm text-neutral-600">Loading Core monitor configuration...</div>
    )}
    {isCoreMonitor && Boolean(error) && (
      <div className="text-sm">Unable to load Core monitor configuration.</div>
    )}
    {isCoreMonitor && config && <CoreMonitorConfigDetails config={config} monitor={monitor} />}
  </div>
);

const CoreMonitorConfigDetails = ({
  config,
  monitor,
}: {
  config: ApiCoreMonitorConfigResponse;
  monitor: ApiMonitorResponse;
}) => (
  <div className="space-y-4">
    <div className="grid gap-3 sm:grid-cols-3">
      <DetailItem label="kind" value={config.kind ?? monitor.type ?? "unknown"} />
      <DetailItem label="interval" value={`${config.interval_seconds ?? 0}s`} />
      {config.kind === "heartbeat" ? (
        <DetailItem label="grace" value={`${coreConfigValue(config, "grace_seconds")}s`} />
      ) : (
        <DetailItem label="timeout" value={`${config.timeout_seconds ?? 0}s`} />
      )}
      <DetailItem
        label="confirmation"
        value={`${config.confirmation_period_seconds ?? 0}s / ${config.confirmation_check_count ?? 0} checks`}
      />
      <DetailItem label="recovery" value={`${config.recovery_period_seconds ?? 0}s`} />
      <DetailItem
        label="maintenance"
        value={`${coreConfigArrayCount(config, "maintenance_windows")} windows`}
      />
      <DetailItem label="paused" value={config.paused ? "yes" : "no"} />
      {config.kind === "heartbeat" && (
        <DetailItem
          label="last signal"
          value={formatDate(config.last_signal_at, DATE_TIME_FORMAT)}
        />
      )}
      <DetailItem label="last run" value={formatDate(config.last_run_at, DATE_TIME_FORMAT)} />
      <DetailItem label="next run" value={formatDate(config.next_run_at, DATE_TIME_FORMAT)} />
    </div>
    {config.kind === "heartbeat" && <HeartbeatSetupPanel config={config} monitor={monitor} />}
    <div className="grid gap-3 lg:grid-cols-2">
      <div className="space-y-2">
        <h3 className="text-sm font-medium">Redacted config</h3>
        <pre className="max-h-80 overflow-auto bg-neutral-950 p-3 text-xs text-neutral-50">
          {formatJSON(config.config)}
        </pre>
      </div>
      <div className="space-y-2">
        <h3 className="text-sm font-medium">Secret refs</h3>
        <pre className="max-h-80 overflow-auto bg-neutral-950 p-3 text-xs text-neutral-50">
          {formatJSON(config.secret_refs)}
        </pre>
      </div>
    </div>
  </div>
);
