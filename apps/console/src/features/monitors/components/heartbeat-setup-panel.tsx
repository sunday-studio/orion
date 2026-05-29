import { Button } from "@/components/ui/button";
import { getApiBase } from "@/lib/custom-instance";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { cn } from "@/lib/utils";
import type { ApiCoreMonitorConfigResponse, ApiMonitorResponse } from "@/orion-sdk";
import { Check, Clipboard, ExternalLink } from "lucide-react";
import { useMemo, useState } from "react";
import { Link } from "react-router-dom";

type HeartbeatSetupPanelProps = {
  className?: string;
  config?: ApiCoreMonitorConfigResponse;
  monitor?: ApiMonitorResponse;
  token?: string;
};

const endpointBase = (token: string) => {
  const base = getApiBase() || (typeof window !== "undefined" ? window.location.origin : "");
  return `${base}/v1/heartbeats/${token}`;
};

const CopyButton = ({ label, value }: { label: string; value: string }) => {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    await navigator.clipboard.writeText(value);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1400);
  };

  return (
    <Button size="sm" variant="outline" onClick={() => void copy()}>
      {copied ? <Check /> : <Clipboard />}
      {copied ? "Copied" : label}
    </Button>
  );
};

const CodeRow = ({ label, value }: { label: string; value: string }) => (
  <div className="space-y-2">
    <div className="flex flex-wrap items-center justify-between gap-2">
      <div className="text-sm font-medium">{label}</div>
      <CopyButton label="Copy" value={value} />
    </div>
    <pre className="overflow-auto bg-neutral-950 p-3 text-xs text-neutral-50">{value}</pre>
  </div>
);

export const HeartbeatSetupPanel = ({
  className,
  config,
  monitor,
  token,
}: HeartbeatSetupPanelProps) => {
  const setup = useMemo(() => {
    if (!token) return undefined;
    const base = endpointBase(token);
    const successEndpoint = `${base}/success`;
    const failureEndpoint = `${base}/failure`;
    return {
      failureCurl: `curl -fsS -X POST ${JSON.stringify(failureEndpoint)} -d '{"job":"backup","status":"failed"}'`,
      failureEndpoint,
      successCurl: `curl -fsS -X POST ${JSON.stringify(successEndpoint)} -d '{"job":"backup","status":"ok"}'`,
      successEndpoint,
    };
  }, [token]);
  const isPending = !config?.last_signal_at;

  return (
    <div className={cn("space-y-4 border border-neutral-200 p-3", className)}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 className="text-sm font-medium">Heartbeat Setup</h3>
          <p className="text-sm text-neutral-600">
            {isPending ? "Waiting for the first signal." : "Latest heartbeat signal recorded."}
          </p>
        </div>
        {monitor?.id && (
          <Link
            className="inline-flex h-8 items-center justify-center gap-2 border border-input bg-background px-3 text-xs font-medium hover:bg-accent hover:text-accent-foreground"
            to={`/monitors/${monitor.id}?tab=config`}
          >
            <ExternalLink className="size-4" />
            Open monitor
          </Link>
        )}
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        <div>
          <div className="text-sm text-neutral-600">last signal</div>
          <div className="text-sm font-medium">{formatDate(config?.last_signal_at, DATE_TIME_FORMAT)}</div>
        </div>
        <div>
          <div className="text-sm text-neutral-600">last success</div>
          <div className="text-sm font-medium">{formatDate(config?.last_success_at, DATE_TIME_FORMAT)}</div>
        </div>
        <div>
          <div className="text-sm text-neutral-600">last failure</div>
          <div className="text-sm font-medium">{formatDate(config?.last_failure_at, DATE_TIME_FORMAT)}</div>
        </div>
      </div>

      {setup ? (
        <div className="grid gap-3 lg:grid-cols-2">
          <CodeRow label="Success endpoint" value={setup.successEndpoint} />
          <CodeRow label="Failure endpoint" value={setup.failureEndpoint} />
          <CodeRow label="Success example" value={setup.successCurl} />
          <CodeRow label="Failure example" value={setup.failureCurl} />
        </div>
      ) : (
        <p className="text-sm text-neutral-600">
          The token is shown after heartbeat monitor creation. Create a replacement heartbeat if a new
          endpoint is needed.
        </p>
      )}
    </div>
  );
};
