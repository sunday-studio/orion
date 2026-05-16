import type { ApiAgentResponse } from "@/orion-sdk";
import { StatusBadge, toStatus } from "@/components/status-badges";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { ArrowRightIcon, ChevronDownIcon, ChevronRightIcon } from "lucide-react";
import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { MonitorList } from "./monitor-list";

export const AgentRow = ({ agent }: { agent: ApiAgentResponse; index: number }) => {
  const navigate = useNavigate();
  const [showMonitors, setShowMonitors] = useState(false);

  const handleShowMonitors = () => setShowMonitors((current) => !current);
  const platform = agent.platform ?? agent.os ?? "unknown";
  const monitorCount = agent.monitor_count ?? 0;
  const status = agent.status ?? (agent.maintenance_mode ? "maintenance" : "unknown");

  const handleClick = () => {
    navigate(`/agents/${agent.id}`);
  };

  return (
    <div className="">
      <button type="button" className="block w-full cursor-pointer group" onClick={handleClick}>
        <div className="grid w-full grid-cols-[1.75rem_minmax(0,1fr)_5.5rem_1.75rem] items-center gap-3 px-1 py-1 text-left text-sm hover:bg-neutral-100 sm:grid-cols-[1.75rem_minmax(10rem,1fr)_7.25rem_8rem_7.5rem_12rem_1.75rem]">
          <button
            type="button"
            onKeyDown={(event) => event.key === "Enter" && handleShowMonitors()}
            onClick={(event) => {
              event.stopPropagation();
              handleShowMonitors();
            }}
            aria-expanded={showMonitors}
            aria-label={showMonitors ? "Hide monitors" : "Show monitors"}
            className="inline-flex size-6 items-center justify-center text-neutral-600 hover:text-lime-700 cursor-pointer"
          >
            {showMonitors ? (
              <ChevronDownIcon className="size-4" />
            ) : (
              <ChevronRightIcon className="size-4" />
            )}
          </button>
          <span className="min-w-0 truncate">{agent.name ?? agent.id ?? "Unknown agent"}</span>
          <span className="hidden min-w-0 sm:inline-flex">
            <StatusBadge value={toStatus(status)} />
          </span>
          <span className="hidden min-w-0 truncate text-neutral-600 sm:inline">{platform}</span>
          <span className="whitespace-nowrap text-right text-neutral-600 sm:text-left">
            {monitorCount} monitors
          </span>
          <span className="hidden min-w-0 truncate text-neutral-600 sm:inline">
            last seen {formatDate(agent.last_seen, DATE_TIME_FORMAT)}
          </span>
          <Link
            to={`/agents/${agent.id}`}
            className="inline-flex size-6 items-center justify-center rounded-full hover:bg-neutral-100 opacity-0 group-hover:opacity-100 transition-opacity"
            aria-label={`Open ${agent.name ?? "agent"} detail`}
          >
            <ArrowRightIcon className="size-4" />
          </Link>
        </div>
      </button>
      {agent.id && showMonitors && (
        <>
          <div className="outline outline-neutral-300/60 rounded-lg overflow-hidden mt-2">
            <div className="px-3 py-2 ">
              <p>Monitors</p>
            </div>
            <div className="overflow-hidden">
              <div className="rounded-t-lg overflow-hidden ">
                <MonitorList agentId={agent.id} />
              </div>
            </div>
          </div>
          <div className="flex flex-col my-4">
            <hr className="opacity-70" />
            <hr className="opacity-60 mt-[-6.5px]!" />
          </div>
        </>
      )}
    </div>
  );
};
