import { FieldPanel, RaisedCard, Switch } from "@/design-system";
import type { ServiceAgentDiscoveryKind } from "@/orion-sdk";
import { kindTitle } from "../detected-model";

const isOff = (kind: ServiceAgentDiscoveryKind) =>
  kind.state === "off" || kind.state === "disabled";

/** Discovery settings: scanning on or off and one switch per kind. Changes are queued for the agent. */
export const DetectedSettings = ({
  enabled,
  kinds,
  canAct,
  onScanning,
  onKind,
}: {
  enabled: boolean;
  kinds: ServiceAgentDiscoveryKind[];
  canAct: boolean;
  onScanning: (enabled: boolean) => void;
  onKind: (kind: string, enabled: boolean) => void;
}) => (
  <FieldPanel title="Discovery" aside="What the agent looks at on this server">
    <RaisedCard>
      <div className="flex min-h-[44px] items-center justify-between gap-3 border-b border-line-soft px-3.5 py-2 text-[13px]">
        <span>
          <b className="font-medium">Scanning</b>
          <small className="block text-xs text-quiet">
            Host metrics keep working when it is off.
          </small>
        </span>
        <Switch
          aria-label="Scanning"
          checked={enabled}
          disabled={!canAct}
          onCheckedChange={onScanning}
        />
      </div>
      {kinds.map((kind) => (
        <div
          key={kind.kind}
          className="flex min-h-[40px] items-center justify-between gap-3 border-t border-line-soft px-3.5 py-2 text-[13px] first:border-t-0"
        >
          <span>
            {kindTitle(kind.kind)}
            {isOff(kind) && <small className="ml-2 text-xs text-faint">Not scanned</small>}
          </span>
          <Switch
            aria-label={kindTitle(kind.kind)}
            checked={!isOff(kind)}
            disabled={!canAct || !enabled}
            onCheckedChange={(next) => onKind(kind.kind ?? "", next)}
          />
        </div>
      ))}
    </RaisedCard>
    {!canAct && (
      <p className="mt-2 text-[13px] text-mid">
        This agent cannot take changes from the Console yet. Update the agent, or change them on the
        server with orion-agent setup.
      </p>
    )}
  </FieldPanel>
);
