import { useState } from "react";
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  useToast,
} from "@/design-system";
import { linkServiceItem, unlinkServiceItem } from "@/orion-sdk";
import type { ServiceServiceDetail, ServiceServiceLinkRef } from "@/orion-sdk";
import { runLinks } from "../link-runner";
import { plural, unlinkParams } from "../services.domain";
import { useRefreshServices } from "../use-service-mutations";
import { LinkPicker, type PickerKind } from "./link-picker";

type Row = { key: string; label: string; hint?: string; ref: ServiceServiceLinkRef };

const rowsOf = (service: ServiceServiceDetail): Record<PickerKind, Row[]> => ({
  monitor: (service.monitors ?? []).map((monitor) => ({
    key: monitor.id ?? "",
    label: monitor.name ?? "",
    hint: monitor.type,
    ref: { kind: "monitor", id: monitor.id },
  })),
  app: (service.apps ?? []).map((app) => ({
    key: app.id ?? "",
    label: app.name ?? app.slug ?? "",
    hint: app.environment,
    ref: { kind: "app", id: app.id },
  })),
  resource: (service.resources ?? []).map((resource) => ({
    key: `${resource.node_id}:${resource.key}`,
    label: resource.name ?? resource.key ?? "",
    hint: resource.kind,
    ref: { kind: "resource", node_id: resource.node_id, key: resource.key },
  })),
});

const SECTIONS: { kind: PickerKind; title: string; add: string }[] = [
  { kind: "monitor", title: "Monitors", add: "Link a monitor" },
  { kind: "app", title: "Apps", add: "Link an app" },
  { kind: "resource", title: "Resources", add: "Link a resource" },
];

/** Link and unlink what a service is made of. Unlinking only clears the link. */
export const EditLinksDialog = ({
  service,
  open,
  onOpenChange,
}: {
  service: ServiceServiceDetail;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) => {
  const serviceId = service.id ?? "";
  const rows = rowsOf(service);
  const [adding, setAdding] = useState<PickerKind | null>(null);
  const [picked, setPicked] = useState<ServiceServiceLinkRef[]>([]);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const refresh = useRefreshServices();
  const toast = useToast();

  const unlink = async (row: Row) => {
    setBusy(`${row.ref.kind}:${row.key}`);
    setError("");
    try {
      await unlinkServiceItem(serviceId, unlinkParams(row.ref));
      refresh(serviceId);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Unable to unlink.");
    } finally {
      setBusy("");
    }
  };

  const link = async () => {
    setBusy("link");
    setError("");
    const outcome = await runLinks(picked, (ref) => linkServiceItem(serviceId, ref));
    setBusy("");
    refresh(serviceId);
    if (outcome.failed.length > 0) {
      setError(outcome.failed.map((failure) => failure.message).join(" "));
      return;
    }
    toast.add({ title: `Linked ${plural(outcome.linked.length, "item")} to ${service.name}` });
    setAdding(null);
    setPicked([]);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100vh-4rem)] max-w-[560px] gap-3.5 overflow-y-auto rounded-2xl p-5">
        <DialogHeader>
          <DialogTitle className="text-[16px]">Edit links for {service.name}</DialogTitle>
          <DialogDescription>
            Unlinking never deletes anything. History stays where it is.
          </DialogDescription>
        </DialogHeader>
        {SECTIONS.map(({ kind, title, add }) => (
          <section key={kind} aria-label={title}>
            <h3 className="mb-0.5 text-xs font-semibold uppercase tracking-[0.02em] text-quiet">
              {title}
            </h3>
            {rows[kind].length === 0 && (
              <p className="py-1.5 text-[12.5px] text-faint">None linked.</p>
            )}
            {rows[kind].map((row) => (
              <div
                key={row.key}
                className="flex items-center justify-between gap-2.5 border-b border-line-soft py-1.5 text-[13px]"
              >
                <span className="min-w-0 truncate">
                  {row.label} {row.hint && <span className="text-faint">{row.hint}</span>}
                </span>
                <Button
                  size="sm"
                  variant="ghost"
                  aria-label={`Unlink ${row.label}`}
                  disabled={busy !== ""}
                  onClick={() => void unlink(row)}
                >
                  Unlink
                </Button>
              </div>
            ))}
            {adding === kind ? (
              <div className="mt-2 grid gap-2">
                <LinkPicker kind={kind} serviceId={serviceId} onChange={setPicked} />
                <div className="flex justify-end gap-2">
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => {
                      setAdding(null);
                      setPicked([]);
                    }}
                  >
                    Cancel
                  </Button>
                  <Button
                    size="sm"
                    disabled={picked.length === 0 || busy !== ""}
                    onClick={() => void link()}
                  >
                    {busy === "link" ? "Linking..." : `Link ${picked.length || ""}`.trim()}
                  </Button>
                </div>
              </div>
            ) : (
              <Button
                size="sm"
                variant="secondary"
                className="mt-2"
                disabled={adding !== null}
                onClick={() => {
                  setAdding(kind);
                  setPicked([]);
                }}
              >
                {add}
              </Button>
            )}
          </section>
        ))}
        {error && (
          <p role="alert" className="text-[12.5px] text-(--tone-down-fg)">
            {error}
          </p>
        )}
        <DialogFooter>
          <Button onClick={() => onOpenChange(false)}>Done</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
