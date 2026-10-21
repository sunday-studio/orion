import { Button, Callout, CodeBlock, useToast } from "@/design-system";
import type { ServiceAgentDiscoveryKind } from "@/orion-sdk";
import { kindTitle } from "../detected-model";

/** A kind the agent could not read: its own message, the exact fix and three ways forward. */
export const DetectedBlocked = ({
  kind,
  canAct,
  onRescan,
  onSkip,
}: {
  kind: ServiceAgentDiscoveryKind;
  canAct: boolean;
  onRescan: () => void;
  onSkip: () => void;
}) => {
  const toast = useToast();
  return (
    <Callout tone="warn" lead={kind.message || `${kindTitle(kind.kind)} could not be read.`}>
      {kind.why && <span className="block">{kind.why}</span>}
      {kind.fix && (
        <>
          <CodeBlock className="mt-2.5">{kind.fix}</CodeBlock>
          <span className="mt-2.5 flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="secondary"
              onClick={() => {
                void navigator.clipboard.writeText(kind.fix ?? "");
                toast.add({ title: "Command copied" });
              }}
            >
              Copy command
            </Button>
            <Button size="sm" variant="secondary" disabled={!canAct} onClick={onRescan}>
              I ran it, rescan
            </Button>
            <Button size="sm" variant="secondary" disabled={!canAct} onClick={onSkip}>
              Skip {kindTitle(kind.kind).toLowerCase()}
            </Button>
          </span>
        </>
      )}
    </Callout>
  );
};
