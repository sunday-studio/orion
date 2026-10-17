import { useState } from "react";
import { Button, Checkbox } from "@/design-system";

type KeyRevealProps = {
  secret: string;
  onDone: () => void;
};

/** One-time secret display (design rule 4): copy it, tick "stored it", then Done. */
export const KeyReveal = ({ secret, onDone }: KeyRevealProps) => {
  const [stored, setStored] = useState(false);
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(secret);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };

  return (
    <section aria-label="New ingest key" className="space-y-3 rounded-[10px] bg-ink p-4 text-page">
      <p className="text-xs">
        This key is shown once. Send it as <span className="font-mono">Authorization: Bearer</span>.
      </p>
      <div className="flex items-center gap-2">
        <code
          className="min-w-0 flex-1 break-all font-mono text-xs"
          data-testid="ingest-key-secret"
        >
          {secret}
        </code>
        <Button size="sm" variant="secondary" onClick={() => void copy()}>
          {copied ? "Copied" : "Copy"}
        </Button>
      </div>
      <label className="flex items-center gap-2 text-[13px]">
        <Checkbox
          aria-label="I stored the key"
          checked={stored}
          onCheckedChange={(next) => setStored(Boolean(next))}
        />
        I stored it somewhere safe
      </label>
      <Button size="sm" disabled={!stored} onClick={onDone}>
        Done
      </Button>
    </section>
  );
};
