import type { ApiMonitorReportResponse } from "@/orion-sdk";
import { formatWhen } from "@/features/overview/overview-time";
import { reportReason } from "../components/incident-detail-utils";

const label = "font-mono text-[10.5px] font-medium uppercase tracking-[0.1em]";

/** Expected vs actual cards; Core will supply the expectation, until then a passing check. */
export const ExpectedActual = ({
  report,
  now,
}: {
  report?: ApiMonitorReportResponse;
  now: number;
}) => {
  const healthy = report?.health === "up";
  const reason = reportReason(report);
  const actual = report
    ? healthy
      ? "Healthy"
      : reason !== "—"
        ? reason
        : report.health
    : "No data";
  return (
    <div className="grid grid-cols-2 gap-2">
      <div className="rounded-[10px] bg-[#142a23] px-4 py-3.5 text-white">
        <span className={`${label} text-[#80ffaa]`}>Expected</span>
        <b className="mt-1 block text-[22px] font-medium tracking-[-0.02em]">Healthy</b>
        <small className="text-xs opacity-75">monitor check passing</small>
      </div>
      <div className="rounded-[10px] border border-line-soft bg-card px-4 py-3.5">
        <span className={`${label} text-faint`}>
          Actual{report ? `, ${formatWhen(report.collected_at ?? report.created_at, now)}` : ""}
        </span>
        <b
          className={`mt-1 block text-[22px] font-medium capitalize tracking-[-0.02em] ${
            healthy || !report ? "" : "text-[#be123c]"
          }`}
        >
          {actual}
        </b>
        <small className="text-xs opacity-75">
          {report ? `latest check reported ${report.health ?? "unknown"}` : "no checks recorded"}
        </small>
      </div>
    </div>
  );
};
