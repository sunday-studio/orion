import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Button, CodeTabs, FieldPanel, Input, Select, useToast } from "@/design-system";
import {
  type ApiMonitorDetailSummary,
  getGetMonitorQueryKey,
  useUpdateCoreMonitor,
} from "@/orion-sdk";
import { formatSpan } from "@/features/overview/overview-time";

const EVERY = [
  [300, "5 minutes"],
  [3600, "1 hour"],
  [21600, "6 hours"],
  [43200, "12 hours"],
  [86400, "24 hours"],
  [604800, "7 days"],
] as const;
const GRACE = [
  [60, "1 minute"],
  [300, "5 minutes"],
  [600, "10 minutes"],
  [1800, "30 minutes"],
  [3600, "1 hour"],
] as const;

const options = (list: readonly (readonly [number, string])[], current?: number) => {
  const known = list.map(([value, label]) => ({ value: String(value), label }));
  return current && !list.some(([value]) => value === current)
    ? [...known, { value: String(current), label: `${current} seconds` }]
    : known;
};

export const MonitorHeartbeat = ({
  monitorId,
  summary,
  now,
}: {
  monitorId: string;
  summary?: ApiMonitorDetailSummary;
  now: number;
}) => {
  const queryClient = useQueryClient();
  const toast = useToast();
  const heartbeat = summary?.heartbeat;
  const [copied, setCopied] = useState(false);
  const update = useUpdateCoreMonitor({
    mutation: {
      onSuccess: () =>
        void queryClient.invalidateQueries({ queryKey: getGetMonitorQueryKey(monitorId) }),
      onError: () => toast.add({ title: "Unable to save the schedule" }),
    },
  });
  const url = heartbeat?.ping_url_success ?? "";
  const snippets = [
    {
      id: "cron",
      label: "Cron",
      code: `0 2 * * * /usr/local/bin/backup.sh && curl -fsS -m 10 ${url}`,
    },
    { id: "curl", label: "curl", code: `curl -fsS -m 10 --retry 3 ${url}` },
    {
      id: "python",
      label: "Python",
      code: `import urllib.request\nurllib.request.urlopen("${url}", timeout=10)`,
    },
    {
      id: "github",
      label: "GitHub Action",
      code: `- name: Ping Orion\n  if: success()\n  run: curl -fsS -m 10 ${url}`,
    },
  ];
  const lastPing = heartbeat?.last_ping_at
    ? `${formatSpan(new Date(heartbeat.last_ping_at).getTime(), now, false, true)} ago`
    : "never";
  return (
    <div className="mt-2 flex flex-col gap-3">
      <FieldPanel title="Ping URLs" aside="Your job calls these">
        <div className="flex flex-col gap-3">
          {(
            [
              ["On success", heartbeat?.ping_url_success],
              ["On failure", heartbeat?.ping_url_failure],
            ] as const
          ).map(([label, value]) => (
            <label key={label} className="flex flex-col gap-1.5 text-[13px] font-medium">
              {label}
              <Input readOnly className="font-normal" value={value ?? ""} />
            </label>
          ))}
        </div>
      </FieldPanel>
      <FieldPanel title="Add to your job" aside="Pick how it runs">
        <CodeTabs aria-label="Job snippet" tabs={snippets} />
        <Button
          size="sm"
          variant="secondary"
          className="mt-3"
          onClick={async () => {
            await navigator.clipboard.writeText(snippets[0]?.code ?? "");
            setCopied(true);
          }}
        >
          {copied ? "Copied" : "Copy snippet"}
        </Button>
      </FieldPanel>
      <FieldPanel title="Schedule" aside={`Last ping ${lastPing}`}>
        <div className="grid grid-cols-2 gap-4">
          <div className="flex flex-col gap-1.5 text-[13px] font-medium">
            Expected every
            <Select
              aria-label="Expected every"
              className="w-full font-normal"
              options={options(EVERY, heartbeat?.expected_every_seconds)}
              value={String(heartbeat?.expected_every_seconds ?? 86400)}
              onValueChange={(next) =>
                update.mutate({ id: monitorId, data: { interval_seconds: Number(next) } })
              }
            />
          </div>
          <div className="flex flex-col gap-1.5 text-[13px] font-medium">
            Grace period
            <Select
              aria-label="Grace period"
              className="w-full font-normal"
              options={options(GRACE, heartbeat?.grace_seconds)}
              value={String(heartbeat?.grace_seconds ?? 600)}
              onValueChange={(next) =>
                update.mutate({ id: monitorId, data: { config: { grace_seconds: Number(next) } } })
              }
            />
          </div>
        </div>
        <p className="mt-3 text-[13px] text-quiet">
          The monitor turns Down after the grace period.
        </p>
      </FieldPanel>
    </div>
  );
};
