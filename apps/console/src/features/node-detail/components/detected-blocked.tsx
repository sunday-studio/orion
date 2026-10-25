import { useState } from "react";
import { Button, Callout, CodeBlock, useToast } from "@/design-system";
import type { ServiceNodeDiscoveryKind } from "@/orion-sdk";
import { kindTitle } from "../detected-model";

/** A kind the node could not read: its own message, the exact fix and three ways forward. */
export const DetectedBlocked = ({
  kind,
  canAct,
  rescanPending,
  skipPending,
  onRescan,
  onSkip,
}: {
  kind: ServiceNodeDiscoveryKind;
  canAct: boolean;
  /** A rescan or a change for this kind is already waiting for the Node. */
  rescanPending?: boolean;
  skipPending?: boolean;
  onRescan: () => void;
  onSkip: () => void;
}) => {
  const toast = useToast();
  const [copied, setCopied] = useState(false);
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
                setCopied(true);
                window.setTimeout(() => setCopied(false), 2000);
              }}
            >
              {copied ? "Copied" : "Copy command"}
            </Button>
            <Button
              size="sm"
              variant="secondary"
              disabled={!canAct || rescanPending}
              onClick={onRescan}
            >
              {rescanPending ? "Rescan requested" : "I ran it, rescan"}
            </Button>
            <Button
              size="sm"
              variant="secondary"
              disabled={!canAct || skipPending}
              onClick={onSkip}
            >
              {skipPending
                ? `Skipping ${kindTitle(kind.kind).toLowerCase()}`
                : `Skip ${kindTitle(kind.kind).toLowerCase()}`}
            </Button>
          </span>
        </>
      )}
    </Callout>
  );
};
