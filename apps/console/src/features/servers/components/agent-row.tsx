import type { ApiAgentResponse } from "@/orion-sdk";
import { StatusBadge, toStatus } from "@/components/status-badges";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { ArrowRightIcon, ChevronDownIcon, ChevronRightIcon } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";
import { MonitorList } from "./monitor-list";

export const AgentRow = ({ agent }: { agent: ApiAgentResponse }) => {
  const [showMonitors, setShowMonitors] = useState(false);

  const handleShowMonitors = () => setShowMonitors((current) => !current);
  const platform = agent.platform ?? agent.os ?? "unknown";
  const monitorCount = agent.monitor_count ?? 0;
  const status = agent.status ?? (agent.maintenance_mode ? "maintenance" : "unknown");

  return (
    <div>
      <div className="grid w-full grid-cols-[1.75rem_minmax(0,1fr)_auto_auto] items-center gap-3 py-2 text-left text-sm sm:grid-cols-[1.75rem_minmax(0,1.4fr)_7rem_7rem_minmax(0,1fr)_auto_auto]">
        <div>
          <button
            type="button"
            onKeyDown={(event) => event.key === "Enter" && handleShowMonitors()}
            onClick={handleShowMonitors}
            aria-expanded={showMonitors}
            aria-label={showMonitors ? "Hide monitors" : "Show monitors"}
            className="inline-flex size-7 items-center justify-center text-neutral-600 hover:text-neutral-950"
          >
            {showMonitors ? (
              <ChevronDownIcon className="size-4" />
            ) : (
              <ChevronRightIcon className="size-4" />
            )}
          </button>
        </div>
        <span className="truncate font-medium">{agent.name ?? agent.id ?? "Unknown agent"}</span>
        <span className="hidden sm:inline">
          <StatusBadge value={toStatus(status)} />
        </span>
        <span className="hidden text-neutral-600 sm:inline">{platform}</span>
        <span className="hidden truncate text-neutral-600 sm:inline">
          last seen {formatDate(agent.last_seen, DATE_TIME_FORMAT)}
        </span>
        <span className="text-right text-neutral-600">{monitorCount} monitors</span>
        <Link
          to={`/agents/${agent.id}`}
          className="inline-flex size-8 items-center justify-center rounded-full hover:bg-neutral-100"
          aria-label={`Open ${agent.name ?? "agent"} detail`}
        >
          <ArrowRightIcon className="size-4" />
        </Link>
      </div>
      {agent.id && showMonitors && (
        <div className="py-2 pl-7">
          <MonitorList agentId={agent.id} />
        </div>
      )}
    </div>
  );
};
