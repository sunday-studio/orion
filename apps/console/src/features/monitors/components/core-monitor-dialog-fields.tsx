import { Checkbox, Input, Select, Textarea } from "@/design-system";
import {
  type FormState,
  apiMethodOptions,
  coreMonitorKindOptions,
  dnsRecordTypeOptions,
  isAPIMethod,
  isCoreMonitorKind,
  isDNSRecordType,
  mailProtocols,
  mailTlsModes,
  pingMethods,
} from "./core-monitor-dialog-state";

type CoreMonitorDialogFieldsProps = {
  advancedConfigError: string;
  form: FormState;
  isHeartbeat: boolean;
  updateForm: (patch: Partial<FormState>) => void;
};

export const CoreMonitorDialogFields = ({
  advancedConfigError,
  form,
  isHeartbeat,
  updateForm,
}: CoreMonitorDialogFieldsProps) => {
  const isURLMonitor =
    form.kind === "http" ||
    form.kind === "http_keyword" ||
    form.kind === "expected_status" ||
    form.kind === "api_request";
  const isHostMonitor = [
    "tcp",
    "udp",
    "dns",
    "tls",
    "ping",
    "mail",
    "smtp",
    "imap",
    "pop",
  ].includes(form.kind);
  const usesAdvancedJSON = form.kind === "synthetic" || form.kind === "playwright";

  return (
    <div className="grid gap-x-4 gap-y-3 sm:grid-cols-2">
      <label className="space-y-1 text-sm">
        <span className="font-medium">Name</span>
        <Input
          required
          value={form.name}
          onChange={(event) => updateForm({ name: event.target.value })}
          placeholder="Core public API"
        />
      </label>
      <label className="space-y-1 text-sm">
        <span className="font-medium">Type</span>
        <Select
          className="w-full"
          value={form.kind}
          onValueChange={(value) => {
            if (isCoreMonitorKind(value)) updateForm({ kind: value });
          }}
          options={coreMonitorKindOptions}
        />
        <select
          aria-label="Core monitor type"
          className="sr-only"
          value={form.kind}
          onChange={(event) => {
            if (isCoreMonitorKind(event.target.value)) updateForm({ kind: event.target.value });
          }}
        >
          {coreMonitorKindOptions.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      </label>
      {isURLMonitor && (
        <>
          <label className="space-y-1 text-sm">
            <span className="font-medium">URL</span>
            <Input
              required
              type="url"
              value={form.url}
              onChange={(event) => updateForm({ url: event.target.value })}
              placeholder="https://example.com/health"
            />
          </label>
          <label className="space-y-1 text-sm">
            <span className="font-medium">Expected status</span>
            <Input
              inputMode="numeric"
              max={599}
              min={100}
              type="number"
              value={form.expectedStatus}
              onChange={(event) => updateForm({ expectedStatus: event.target.value })}
            />
          </label>
        </>
      )}
      {form.kind === "api_request" && (
        <>
          <label className="space-y-1 text-sm">
            <span className="font-medium">Method</span>
            <Select
              className="w-full"
              value={form.apiMethod}
              onValueChange={(value) => {
                if (isAPIMethod(value)) updateForm({ apiMethod: value });
              }}
              options={apiMethodOptions.map((value) => ({ value, label: value }))}
            />
          </label>
          <label className="space-y-1 text-sm">
            <span className="font-medium">Expected statuses</span>
            <Input
              value={form.expectedStatuses}
              onChange={(event) => updateForm({ expectedStatuses: event.target.value })}
              placeholder="200, 201"
            />
          </label>
          <label className="space-y-1 text-sm sm:col-span-2">
            <span className="font-medium">Headers</span>
            <Textarea
              value={form.apiHeaders}
              onChange={(event) => updateForm({ apiHeaders: event.target.value })}
              placeholder="X-Trace: trace-1"
            />
          </label>
          <label className="space-y-1 text-sm sm:col-span-2">
            <span className="font-medium">Body</span>
            <Textarea
              value={form.apiBody}
              onChange={(event) => updateForm({ apiBody: event.target.value })}
              placeholder='{"status":"check"}'
            />
          </label>
          <label className="space-y-1 text-sm sm:col-span-2">
            <span className="font-medium">JSON assertions</span>
            <Textarea
              value={form.apiJSONAssertions}
              onChange={(event) => updateForm({ apiJSONAssertions: event.target.value })}
              placeholder='[{"path":"$.ok","equals":true}]'
            />
          </label>
        </>
      )}
      {isHostMonitor && (
        <>
          <label className="space-y-1 text-sm">
            <span className="font-medium">Host</span>
            <Input
              required
              value={form.host}
              onChange={(event) => updateForm({ host: event.target.value })}
              placeholder="api.example.com"
            />
          </label>
          {["tcp", "udp", "tls", "ping", "mail", "smtp", "imap", "pop"].includes(form.kind) && (
            <label className="space-y-1 text-sm">
              <span className="font-medium">Port</span>
              <Input
                required={form.kind === "tcp" || form.kind === "udp"}
                inputMode="numeric"
                max={65535}
                min={1}
                type="number"
                value={form.port}
                onChange={(event) => updateForm({ port: event.target.value })}
                placeholder={form.kind === "tls" || form.kind === "ping" ? "443" : "5432"}
              />
            </label>
          )}
        </>
      )}
      {form.kind === "udp" && (
        <>
          <label className="space-y-1 text-sm">
            <span className="font-medium">Payload</span>
            <Input
              value={form.udpPayload}
              onChange={(event) => updateForm({ udpPayload: event.target.value })}
              placeholder="optional UDP payload"
            />
          </label>
          <label className="space-y-1 text-sm">
            <span className="font-medium">Expected response</span>
            <Input
              value={form.udpExpectedResponse}
              onChange={(event) => updateForm({ udpExpectedResponse: event.target.value })}
              placeholder="optional response text"
            />
          </label>
        </>
      )}
      {form.kind === "dns" && (
        <>
          <label className="space-y-1 text-sm">
            <span className="font-medium">Record type</span>
            <Select
              className="w-full"
              value={form.recordType}
              onValueChange={(value) => {
                if (isDNSRecordType(value)) updateForm({ recordType: value });
              }}
              options={dnsRecordTypeOptions.map((value) => ({ value, label: value }))}
            />
          </label>
          <label className="space-y-1 text-sm sm:col-span-2">
            <span className="font-medium">Expected values</span>
            <Textarea
              value={form.expectedValues}
              onChange={(event) => updateForm({ expectedValues: event.target.value })}
              placeholder="203.0.113.10"
            />
          </label>
        </>
      )}
      {form.kind === "tls" && (
        <label className="space-y-1 text-sm">
          <span className="font-medium">Server name</span>
          <Input
            value={form.serverName}
            onChange={(event) => updateForm({ serverName: event.target.value })}
            placeholder="api.example.com"
          />
        </label>
      )}
      {form.kind === "ping" && (
        <label className="space-y-1 text-sm">
          <span className="font-medium">Method</span>
          <Select
            className="w-full"
            value={form.pingMethod}
            onValueChange={(value) => updateForm({ pingMethod: value })}
            options={pingMethods.map((method) => ({ value: method, label: method.toUpperCase() }))}
          />
        </label>
      )}
      {["mail", "smtp", "imap", "pop"].includes(form.kind) && (
        <>
          {form.kind === "mail" && (
            <label className="space-y-1 text-sm">
              <span className="font-medium">Protocol</span>
              <Select
                className="w-full"
                value={form.mailProtocol}
                onValueChange={(value) => updateForm({ mailProtocol: value })}
                options={mailProtocols.map((protocol) => ({
                  value: protocol,
                  label: protocol.toUpperCase(),
                }))}
              />
            </label>
          )}
          <label className="space-y-1 text-sm">
            <span className="font-medium">TLS mode</span>
            <Select
              className="w-full"
              value={form.mailTlsMode}
              onValueChange={(value) => updateForm({ mailTlsMode: value })}
              options={mailTlsModes.map((value) => ({ value, label: value }))}
            />
          </label>
        </>
      )}
      {(form.kind === "tls" || form.kind === "domain_expiration") && (
        <label className="space-y-1 text-sm">
          <span className="font-medium">Warning days</span>
          <Input
            inputMode="numeric"
            min={0}
            type="number"
            value={form.warningDays}
            onChange={(event) => updateForm({ warningDays: event.target.value })}
          />
        </label>
      )}
      {form.kind === "domain_expiration" && (
        <>
          <label className="space-y-1 text-sm">
            <span className="font-medium">Domain</span>
            <Input
              required
              value={form.domain}
              onChange={(event) => updateForm({ domain: event.target.value })}
              placeholder="example.com"
            />
          </label>
          <label className="space-y-1 text-sm">
            <span className="font-medium">RDAP URL</span>
            <Input
              type="url"
              value={form.rdapUrl}
              onChange={(event) => updateForm({ rdapUrl: event.target.value })}
              placeholder="https://rdap.example.com/domain/example.com"
            />
          </label>
          <label className="space-y-1 text-sm">
            <span className="font-medium">WHOIS server</span>
            <Input
              value={form.whoisServer}
              onChange={(event) => updateForm({ whoisServer: event.target.value })}
              placeholder="whois.example.com:43"
            />
          </label>
        </>
      )}
      {usesAdvancedJSON && (
        <label className="space-y-1 text-sm sm:col-span-2">
          <span className="font-medium">Configuration JSON</span>
          <Textarea
            value={form.advancedConfig}
            onChange={(event) => updateForm({ advancedConfig: event.target.value })}
            rows={10}
            spellCheck={false}
          />
          {advancedConfigError && (
            <span className="block text-rose-700">{advancedConfigError}</span>
          )}
        </label>
      )}
      <label className="space-y-1 text-sm">
        <span className="font-medium">Interval seconds</span>
        <Input
          inputMode="numeric"
          min={10}
          type="number"
          value={form.intervalSeconds}
          onChange={(event) => updateForm({ intervalSeconds: event.target.value })}
        />
      </label>
      {isHeartbeat ? (
        <label className="space-y-1 text-sm">
          <span className="font-medium">Grace seconds</span>
          <Input
            inputMode="numeric"
            min={0}
            type="number"
            value={form.graceSeconds}
            onChange={(event) => updateForm({ graceSeconds: event.target.value })}
          />
        </label>
      ) : (
        <label className="space-y-1 text-sm">
          <span className="font-medium">Timeout seconds</span>
          <Input
            inputMode="numeric"
            min={1}
            type="number"
            value={form.timeoutSeconds}
            onChange={(event) => updateForm({ timeoutSeconds: event.target.value })}
          />
        </label>
      )}
      <label className="space-y-1 text-sm">
        <span className="font-medium">Confirmation seconds</span>
        <Input
          inputMode="numeric"
          min={0}
          type="number"
          value={form.confirmationPeriodSeconds}
          onChange={(event) => updateForm({ confirmationPeriodSeconds: event.target.value })}
        />
      </label>
      <label className="space-y-1 text-sm">
        <span className="font-medium">Confirmation checks</span>
        <Input
          inputMode="numeric"
          min={0}
          type="number"
          value={form.confirmationCheckCount}
          onChange={(event) => updateForm({ confirmationCheckCount: event.target.value })}
        />
      </label>
      <label className="space-y-1 text-sm">
        <span className="font-medium">Recovery seconds</span>
        <Input
          inputMode="numeric"
          min={0}
          type="number"
          value={form.recoveryPeriodSeconds}
          onChange={(event) => updateForm({ recoveryPeriodSeconds: event.target.value })}
        />
      </label>
      <label className="flex items-center gap-2 pt-7 text-sm">
        <Checkbox
          checked={form.paused}
          onCheckedChange={(checked) => updateForm({ paused: checked === true })}
        />
        Start paused
      </label>
      {form.kind === "http_keyword" && (
        <label className="space-y-1 text-sm sm:col-span-2">
          <span className="font-medium">Required response text</span>
          <Textarea
            value={form.requiredContains}
            onChange={(event) => updateForm({ requiredContains: event.target.value })}
            placeholder="One keyword per line"
          />
        </label>
      )}
      <label className="space-y-1 text-sm sm:col-span-2">
        <span className="font-medium">Description</span>
        <Textarea
          value={form.description}
          onChange={(event) => updateForm({ description: event.target.value })}
          placeholder="What this monitor protects"
        />
      </label>
    </div>
  );
};
