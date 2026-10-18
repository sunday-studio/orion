import { useState } from "react";
import {
  Button,
  Callout,
  Checkbox,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  useToast,
} from "@/design-system";
import { useCreateIngestKey } from "@/orion-sdk";
import { Labeled } from "@/features/settings/components-ui";

/** One-time secret: copy it, tick "stored it", then Done. */
export const KeyRevealDialog = ({
  title,
  secret,
  note,
  onDone,
}: {
  title: string;
  secret: string;
  note?: string;
  onDone: () => void;
}) => {
  const toast = useToast();
  const [stored, setStored] = useState(false);
  return (
    <Dialog open onOpenChange={() => undefined}>
      <DialogContent className="max-w-[500px] gap-3.5 rounded-2xl p-5" showCloseButton={false}>
        <DialogHeader>
          <DialogTitle className="text-[16px]">{title}</DialogTitle>
        </DialogHeader>
        <code
          data-testid="ingest-key-secret"
          className="block break-all rounded-lg bg-[#142a23] px-3 py-3 font-mono text-[12.5px] text-[#c8f5dc]"
        >
          {secret}
        </code>
        <div>
          <Button
            size="sm"
            onClick={() => {
              void navigator.clipboard.writeText(secret);
              toast.add({ title: "Key copied" });
            }}
          >
            Copy key
          </Button>
        </div>
        <Callout tone="down">
          Copy it now. Orion stores only a hash, so it cannot be shown again.
        </Callout>
        {note && <p className="text-[13px] text-quiet">{note}</p>}
        <label className="flex items-center gap-2 text-[13px]">
          <Checkbox checked={stored} onCheckedChange={(next) => setStored(Boolean(next))} />I have
          stored this key
        </label>
        <DialogFooter>
          <Button disabled={!stored} onClick={onDone}>
            Done
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

/** Names a new ingest key, then reveals it once. */
export const NewKeyDialog = ({
  appId,
  open,
  onOpenChange,
  onCreated,
}: {
  appId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: () => void;
}) => {
  const [name, setName] = useState("");
  const [secret, setSecret] = useState("");
  const [error, setError] = useState<string>();
  const create = useCreateIngestKey({
    mutation: {
      onSuccess: (result) => {
        setSecret(result.key?.key ?? "");
        onCreated();
      },
      onError: (failure) => setError((failure as Error).message || "Unable to create the key."),
    },
  });
  const finish = () => {
    setName("");
    setSecret("");
    setError(undefined);
    onOpenChange(false);
  };
  if (secret) return <KeyRevealDialog title="New ingest key" secret={secret} onDone={finish} />;
  return (
    <Dialog open={open} onOpenChange={(next) => (next ? onOpenChange(true) : finish())}>
      <DialogContent className="max-w-[440px] gap-3.5 rounded-2xl p-5">
        <DialogHeader>
          <DialogTitle className="text-[16px]">New ingest key</DialogTitle>
        </DialogHeader>
        <Labeled label="Name" error={error}>
          <Input
            placeholder="production server"
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
        </Labeled>
        <DialogFooter>
          <Button variant="secondary" onClick={finish}>
            Cancel
          </Button>
          <Button
            disabled={create.isPending || !name.trim()}
            onClick={() => create.mutate({ id: appId, data: { name: name.trim() } })}
          >
            Create key
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
