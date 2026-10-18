import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  AnimatedHeight,
  Button,
  Callout,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Select,
} from "@/design-system";
import { createIngestKey, getListAppsQueryKey, useCreateApp } from "@/orion-sdk";
import { Labeled } from "@/features/settings/components-ui";
import { KeyRevealDialog } from "./key-dialogs";

const KEEP = [
  { value: "7", label: "7 days" },
  { value: "14", label: "14 days (default)" },
  { value: "30", label: "30 days" },
  { value: "90", label: "90 days" },
];
const ENVIRONMENTS = ["production", "staging", "development"].map((value) => ({
  value,
  label: value,
}));

const slugOf = (name: string) =>
  name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");

/** Creates an app and its first ingest key, shown once. */
export const NewAppDialog = ({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("production");
  const [keep, setKeep] = useState("14");
  const [error, setError] = useState<string>();
  const [created, setCreated] = useState<{ appId: string; secret: string }>();
  const create = useCreateApp({
    mutation: {
      onSuccess: async (result) => {
        const appId = result.app?.id ?? "";
        void queryClient.invalidateQueries({ queryKey: getListAppsQueryKey() });
        try {
          const key = await createIngestKey(appId, { name: "default" });
          setCreated({ appId, secret: key.key?.key ?? "" });
        } catch {
          onOpenChange(false);
          navigate(`/settings/apps/${appId}`);
        }
      },
      onError: (failure) => setError((failure as Error).message || "Unable to create the app."),
    },
  });
  const reset = () => {
    setName("");
    setError(undefined);
    setCreated(undefined);
  };
  return (
    <>
      <Dialog
        open={open && !created}
        onOpenChange={(next) => {
          onOpenChange(next);
          if (!next) reset();
        }}
      >
        <DialogContent className="max-w-[480px] rounded-2xl p-5">
          <AnimatedHeight>
            <div className="grid gap-3.5 pb-1.5">
              <DialogHeader>
                <DialogTitle className="text-[16px]">New app</DialogTitle>
              </DialogHeader>
              <Labeled label="Name" hint={name ? `Address: ${slugOf(name)}` : undefined}>
                <Input
                  placeholder="billing-api"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                />
              </Labeled>
              <div className="grid grid-cols-2 gap-3">
                <Labeled label="Environment">
                  <Select
                    aria-label="Environment"
                    className="w-full"
                    options={ENVIRONMENTS}
                    value={environment}
                    onValueChange={setEnvironment}
                  />
                </Labeled>
                <Labeled label="Keep logs for">
                  <Select
                    aria-label="Keep logs for"
                    className="w-full"
                    options={KEEP}
                    value={keep}
                    onValueChange={setKeep}
                  />
                </Labeled>
              </div>
              <p className="text-[13px] text-quiet">
                You get an ingest key next. It is shown once.
              </p>
              {error && <Callout tone="down">{error}</Callout>}
              <DialogFooter>
                <Button variant="secondary" onClick={() => onOpenChange(false)}>
                  Cancel
                </Button>
                <Button
                  disabled={create.isPending || !name.trim()}
                  onClick={() => {
                    setError(undefined);
                    create.mutate({
                      data: {
                        name: slugOf(name) || name.trim(),
                        environment,
                        retention_days: Number(keep),
                      },
                    });
                  }}
                >
                  Create app
                </Button>
              </DialogFooter>
            </div>
          </AnimatedHeight>
        </DialogContent>
      </Dialog>
      {created && (
        <KeyRevealDialog
          title="New ingest key"
          secret={created.secret}
          onDone={() => {
            const appId = created.appId;
            reset();
            onOpenChange(false);
            navigate(`/settings/apps/${appId}`);
          }}
        />
      )}
    </>
  );
};
