import type { ApiIncidentExpectation, ApiMonitorReportResponse } from "@/orion-sdk";
import { formatWhen } from "@/features/overview/overview-time";
import { reportReason } from "../components/incident-detail-utils";

const label = "font-mono text-[10.5px] font-medium uppercase tracking-[0.1em]";

/** Expected vs actual cards; Core supplies both, the latest check fills in for older records. */
export const ExpectedActual = ({
  expected,
  actual,
  report,
  now,
}: {
  expected?: ApiIncidentExpectation;
  actual?: ApiIncidentExpectation;
  report?: ApiMonitorReportResponse;
  now: number;
}) => {
  const healthy = report?.health === "up";
  const reason = reportReason(report);
  const actualTitle =
    actual?.title ??
    (report ? (healthy ? "Healthy" : reason !== "—" ? reason : report.health) : "No data");
  const actualAt = actual?.at ?? report?.collected_at ?? report?.created_at;
  const failing = !healthy && Boolean(actual ?? report);
  return (
    <div className="grid grid-cols-2 gap-2">
      <div className="rounded-[10px] bg-[#142a23] px-4 py-3.5 text-white">
        <span className={`${label} text-[#80ffaa]`}>Expected</span>
        <b className="mt-1 block text-[22px] font-medium tracking-[-0.02em]">
          {expected?.title ?? "Healthy"}
        </b>
        <small className="text-xs opacity-75">{expected?.detail ?? "monitor check passing"}</small>
      </div>
      <div className="rounded-[10px] border border-line-soft bg-card px-4 py-3.5">
        <span className={`${label} text-faint`}>
          Actual{actualAt ? `, ${formatWhen(actualAt, now)}` : ""}
        </span>
        <b
          className={`mt-1 block text-[22px] font-medium capitalize tracking-[-0.02em] ${
            failing ? "text-[#be123c]" : ""
          }`}
        >
          {actualTitle}
        </b>
        <small className="text-xs opacity-75">
          {actual?.detail ??
            (report ? `latest check reported ${report.health ?? "unknown"}` : "no checks recorded")}
        </small>
      </div>
    </div>
  );
};
