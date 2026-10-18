import { type FormEvent, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  Button,
  Callout,
  ChoiceTiles,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Select,
} from "@/design-system";
import { type ServiceCoreManagedMonitorCreateRequest, useTestRunUnsavedMonitor } from "@/orion-sdk";
import { MonitorTestResultDialog } from "./monitor-test-result-dialog";
import {
  type FormState,
  buildConfigPayload,
  defaultForm,
} from "../components/core-monitor-dialog-state";

type Kind = "http" | "tcp" | "heartbeat" | "agent";

const TILES = [
  { value: "http", title: "HTTP", caption: "Website, API" },
  { value: "tcp", title: "TCP", caption: "Port" },
  { value: "heartbeat", title: "Heartbeat", caption: "Cron job" },
  { value: "agent", title: "Agent", caption: "On a server" },
] as const;

const seconds = (values: readonly number[], unit: "seconds" | "minutes" | "hours" = "seconds") =>
  values.map((value) => ({
    value: String(value),
    label:
      unit === "seconds" && value >= 3600
        ? `${value / 3600} ${value / 3600 === 1 ? "hour" : "hours"}`
        : unit === "seconds" && value >= 60
          ? `${value / 60} ${value / 60 === 1 ? "minute" : "minutes"}`
          : `${value} seconds`,
  }));

const EVERY = seconds([30, 60, 120, 300, 600, 1800, 3600, 86400]);
const TIMEOUT = seconds([5, 10, 15, 30, 60]);
const GRACE = seconds([60, 300, 600, 1800, 3600]);
const FAIL_AFTER = [1, 2, 3, 5].map((value) => ({
  value: String(value),
  label: `${value} ${value === 1 ? "check" : "checks"}`,
}));

const label = "flex min-w-0 flex-col gap-1.5 text-[13px] font-medium";
const hint = "text-[13px] font-normal text-quiet";

type Props = {
  open: boolean;
  error?: string;
  pending?: boolean;
  onOpenChange: (open: boolean) => void;
  onCreate: (payload: ServiceCoreManagedMonitorCreateRequest) => void;
  /** Opens the full editor for the other check types (DNS, TLS, mail, API request, ...). */
  onMoreTypes: () => void;
};

export const MonitorCreateDialog = ({
  open,
  error,
  pending,
  onOpenChange,
  onCreate,
  onMoreTypes,
}: Props) => {
  const [kind, setKind] = useState<Kind>("http");
  const [form, setForm] = useState<FormState>(defaultForm);
  const [failAfter, setFailAfter] = useState("3");
  const [localError, setLocalError] = useState("");
  const testRun = useTestRunUnsavedMonitor();
  const { reset: resetTest } = testRun;

  useEffect(() => {
    if (!open) return;
    setKind("http");
    setForm({ ...defaultForm, kind: "http", intervalSeconds: "30" });
    setFailAfter("3");
    setLocalError("");
    resetTest();
  }, [open, resetTest]);

  const patch = (next: Partial<FormState>) => setForm((current) => ({ ...current, ...next }));
  const pick = (next: string) => {
    const value = next as Kind;
    setKind(value);
    if (value === "agent") return;
    patch({
      kind: value,
      intervalSeconds: value === "http" ? "30" : value === "tcp" ? "60" : "86400",
      timeoutSeconds: value === "tcp" ? "5" : "10",
      graceSeconds: "600",
    });
  };

  const valid =
    kind !== "agent" &&
    form.name.trim() !== "" &&
    (kind === "http"
      ? form.url.trim() !== ""
      : kind === "tcp"
        ? form.host.trim() !== "" && form.port.trim() !== ""
        : true);

  const runTest = () => {
    try {
      testRun.mutate({
        data: {
          kind: form.kind,
          config: buildConfigPayload(form),
          timeout_seconds: Number(form.timeoutSeconds),
        },
      });
    } catch (failure) {
      setLocalError(failure instanceof Error ? failure.message : "The monitor is invalid.");
    }
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!valid) return;
    try {
      const heartbeat = kind === "heartbeat";
      onCreate({
        config: buildConfigPayload(form),
        confirmation_check_count: heartbeat ? 0 : Number(failAfter),
        interval_seconds: Number(form.intervalSeconds),
        kind: form.kind,
        name: form.name.trim(),
        ...(heartbeat ? {} : { timeout_seconds: Number(form.timeoutSeconds) }),
        type: form.kind,
      });
    } catch (failure) {
      setLocalError(failure instanceof Error ? failure.message : "The monitor is invalid.");
    }
  };

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="max-w-[540px] gap-3.5 rounded-2xl p-5">
          <form className="flex min-w-0 flex-col gap-3.5" onSubmit={submit}>
            <DialogHeader>
              <DialogTitle className="text-[15px] font-medium">Create monitor</DialogTitle>
            </DialogHeader>
            <ChoiceTiles
              aria-label="Monitor type"
              tiles={TILES}
              value={kind}
              onValueChange={pick}
            />
            {kind === "agent" ? (
              <Callout tone="warn" className="leading-[1.5]">
                Agent monitors run on a server through its agent. Pick the server and what to check
                on the{" "}
                <Link
                  className="underline"
                  to="/monitors/new/agent"
                  onClick={() => onOpenChange(false)}
                >
                  agent monitor page
                </Link>
                .
              </Callout>
            ) : (
              <>
                <label className={label}>
                  Name
                  <Input
                    className="font-normal"
                    value={form.name}
                    onChange={(event) => patch({ name: event.target.value })}
                    placeholder={
                      kind === "tcp"
                        ? "pg-tcp"
                        : kind === "heartbeat"
                          ? "nightly-backup"
                          : "api-https"
                    }
                  />
                </label>
                {kind === "http" && (
                  <label className={label}>
                    URL
                    <Input
                      className="font-normal"
                      value={form.url}
                      onChange={(event) => patch({ url: event.target.value })}
                      placeholder="https://api.example.com/health"
                    />
                    <span className={hint}>
                      Core sends a GET request and expects a 200 response.
                    </span>
                  </label>
                )}
                {kind === "tcp" && (
                  <div className="grid grid-cols-[1fr_96px] gap-3">
                    <label className={label}>
                      Host
                      <Input
                        className="font-normal"
                        value={form.host}
                        onChange={(event) => patch({ host: event.target.value })}
                        placeholder="db-02.internal"
                      />
                    </label>
                    <label className={label}>
                      Port
                      <Input
                        className="font-normal"
                        inputMode="numeric"
                        value={form.port}
                        onChange={(event) => patch({ port: event.target.value })}
                        placeholder="5432"
                      />
                    </label>
                  </div>
                )}
                <div
                  className={
                    kind === "heartbeat" ? "grid grid-cols-2 gap-3" : "grid grid-cols-3 gap-3"
                  }
                >
                  <div className={label}>
                    {kind === "heartbeat" ? "Expected every" : "Check every"}
                    <Select
                      aria-label={kind === "heartbeat" ? "Expected every" : "Check every"}
                      className="w-full font-normal"
                      options={EVERY}
                      value={form.intervalSeconds}
                      onValueChange={(next) => patch({ intervalSeconds: next })}
                    />
                  </div>
                  {kind === "heartbeat" ? (
                    <div className={label}>
                      Grace period
                      <Select
                        aria-label="Grace period"
                        className="w-full font-normal"
                        options={GRACE}
                        value={form.graceSeconds}
                        onValueChange={(next) => patch({ graceSeconds: next })}
                      />
                    </div>
                  ) : (
                    <div className={label}>
                      Timeout
                      <Select
                        aria-label="Timeout"
                        className="w-full font-normal"
                        options={TIMEOUT}
                        value={form.timeoutSeconds}
                        onValueChange={(next) => patch({ timeoutSeconds: next })}
                      />
                    </div>
                  )}
                  {kind !== "heartbeat" && (
                    <div className={label}>
                      Fail after
                      <Select
                        aria-label="Fail after"
                        className="w-full font-normal"
                        options={FAIL_AFTER}
                        value={failAfter}
                        onValueChange={setFailAfter}
                      />
                    </div>
                  )}
                </div>
                {kind === "tcp" && (
                  <p className={hint}>
                    Orion opens a TCP connection and expects it to succeed. It sends no data.
                  </p>
                )}
                {kind === "heartbeat" && (
                  <p className={hint}>
                    Your job pings a URL when it runs. The monitor turns Down after the grace
                    period.
                  </p>
                )}
              </>
            )}
            {(error || localError) && <Callout tone="warn">{error || localError}</Callout>}
            {testRun.isError && <Callout tone="warn">Unable to run the test. Try again.</Callout>}
            <DialogFooter className="items-center sm:justify-between">
              <div className="flex items-center gap-1">
                <Button
                  type="button"
                  variant="ghost"
                  disabled={!valid || kind === "heartbeat" || testRun.isPending}
                  onClick={runTest}
                >
                  Test now
                </Button>
                <Button type="button" variant="ghost" onClick={onMoreTypes}>
                  More types
                </Button>
              </div>
              <div className="flex gap-2">
                <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
                  Cancel
                </Button>
                <Button type="submit" disabled={!valid || pending}>
                  Create monitor
                </Button>
              </div>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      <MonitorTestResultDialog
        result={testRun.data}
        running={testRun.isPending}
        onRunAgain={runTest}
        onClose={() => testRun.reset()}
      />
    </>
  );
};
