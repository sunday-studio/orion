import { useState } from "react";
import {
  AnimatedHeight,
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Segmented,
  useToast,
} from "@/design-system";
import { useCreateAPIToken } from "@/orion-sdk";
import { Labeled } from "../components-ui";

/** Creates an API token; the secret is shown once, inside the same dialog. */
export const TokenDialog = ({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: () => void;
}) => {
  const toast = useToast();
  const [name, setName] = useState("");
  const [scope, setScope] = useState("read");
  const [secret, setSecret] = useState("");
  const [error, setError] = useState<string>();
  const create = useCreateAPIToken({
    mutation: {
      onSuccess: (result) => {
        setSecret(result.token?.token ?? "");
        onCreated();
      },
      onError: (failure) => setError((failure as Error).message || "Unable to create the token."),
    },
  });
  const close = (next: boolean) => {
    onOpenChange(next);
    if (!next) {
      setName("");
      setScope("read");
      setSecret("");
      setError(undefined);
    }
  };
  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="max-w-[480px] rounded-2xl p-5">
        <AnimatedHeight>
          <div className="grid gap-3.5 pb-1.5">
            <DialogHeader>
              <DialogTitle className="text-[16px]">New API token</DialogTitle>
            </DialogHeader>
            <Labeled label="Name" error={error}>
              <Input
                placeholder="ci-deploys"
                value={name}
                disabled={Boolean(secret)}
                onChange={(event) => setName(event.target.value)}
              />
            </Labeled>
            <div className="grid gap-1.5">
              <span className="text-[12.5px] font-medium">Scope</span>
              <Segmented
                aria-label="Scope"
                surface="card"
                options={[
                  { value: "read", label: "Read" },
                  { value: "write", label: "Read and write" },
                ]}
                value={scope}
                onValueChange={(next) => !secret && setScope(next)}
              />
            </div>
            {secret && (
              <>
                <code className="block break-all rounded-lg bg-[#142a23] px-3 py-3 font-mono text-[12.5px] text-[#c8f5dc]">
                  {secret}
                </code>
                <p className="text-[13px] text-quiet">Copy it now. Orion only shows it once.</p>
              </>
            )}
            <DialogFooter>
              <Button variant="secondary" onClick={() => close(false)}>
                {secret ? "Close" : "Cancel"}
              </Button>
              {secret ? (
                <Button
                  onClick={() => {
                    void navigator.clipboard.writeText(secret);
                    toast.add({ title: "Token copied" });
                  }}
                >
                  Copy token
                </Button>
              ) : (
                <Button
                  disabled={create.isPending || !name.trim()}
                  onClick={() => {
                    setError(undefined);
                    create.mutate({
                      data: { name: name.trim(), scope: scope as "read" | "write" },
                    });
                  }}
                >
                  Create token
                </Button>
              )}
            </DialogFooter>
          </div>
        </AnimatedHeight>
      </DialogContent>
    </Dialog>
  );
};
