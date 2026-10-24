import { PlusIcon, XIcon } from "lucide-react";
import { useState } from "react";
import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Select,
  useToast,
} from "@/design-system";
import {
  type ApiStatusPageComponentResponse,
  type ApiStatusPageDetailResponse,
  useCreateStatusPageComponent,
  useCreateStatusPageComponentMapping,
  useDeleteStatusPageComponent,
  useDeleteStatusPageComponentMapping,
  useGetMonitors,
  useGetNodes,
  useUpdateStatusPageComponent,
} from "@/orion-sdk";
import { componentPayload } from "./component-payload";

export const NO_SECTION = "";

const KIND_LABEL: Record<string, string> = { monitor: "Monitor", node: "Server" };

export const ComponentDialog = ({
  pageId,
  detail,
  component,
  open,
  onOpenChange,
  onChanged,
}: {
  pageId: string;
  detail?: ApiStatusPageDetailResponse;
  component?: ApiStatusPageComponentResponse;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onChanged: () => void;
}) => {
  const toast = useToast();
  const [name, setName] = useState(component?.public_name ?? "");
  const [sectionId, setSectionId] = useState(component?.section_id ?? NO_SECTION);
  const [kind, setKind] = useState("monitor");
  const [resourceId, setResourceId] = useState("");
  const monitors = useGetMonitors({ limit: 200 }, { query: { enabled: open } });
  const nodes = useGetNodes({ limit: 200 }, { query: { enabled: open } });
  const done = () => {
    onChanged();
    onOpenChange(false);
  };
  const fail = () => toast.add({ title: "Unable to save the component" });
  const create = useCreateStatusPageComponent({ mutation: { onSuccess: done, onError: fail } });
  const update = useUpdateStatusPageComponent({ mutation: { onSuccess: done, onError: fail } });
  const remove = useDeleteStatusPageComponent({ mutation: { onSuccess: done, onError: fail } });
  const addMapping = useCreateStatusPageComponentMapping({
    mutation: { onSuccess: onChanged, onError: fail },
  });
  const removeMapping = useDeleteStatusPageComponentMapping({
    mutation: { onSuccess: onChanged, onError: fail },
  });
  const monitorOptions = (monitors.data?.monitors ?? []).map((m) => ({
    value: m.id ?? "",
    label: m.name ?? "",
  }));
  const nodeOptions = (nodes.data?.nodes ?? []).map((n) => ({
    value: n.id ?? "",
    label: n.name ?? "",
  }));
  const names = new Map([...monitorOptions, ...nodeOptions].map((o) => [o.value, o.label]));
  const mappings = component?.mappings ?? [];
  const linked = new Set(mappings.map((m) => m.resource_id));
  const options = (kind === "monitor" ? monitorOptions : nodeOptions).filter(
    (o) => !linked.has(o.value),
  );
  const save = () => {
    const data = component
      ? componentPayload(component, {
          public_name: name.trim(),
          section_id: sectionId || undefined,
        })
      : { public_name: name.trim(), section_id: sectionId || undefined, visible: true };
    if (component?.id) update.mutate({ id: pageId, componentId: component.id, data });
    else create.mutate({ id: pageId, data });
  };
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[520px] gap-4 rounded-2xl p-5">
        <DialogHeader>
          <DialogTitle>{component ? "Edit component" : "Add component"}</DialogTitle>
        </DialogHeader>
        <label className="grid gap-1.5 text-[12.5px] font-medium text-mid">
          Name
          <Input value={name} onChange={(event) => setName(event.target.value)} />
        </label>
        <div className="grid gap-1.5 text-[12.5px] font-medium text-mid">
          Section
          <Select
            aria-label="Section"
            className="w-full"
            options={[
              { value: NO_SECTION, label: "No section" },
              ...(detail?.sections ?? []).map((s) => ({ value: s.id ?? "", label: s.name ?? "" })),
            ]}
            value={sectionId}
            onValueChange={setSectionId}
          />
        </div>
        {component?.id && (
          <div className="grid gap-2">
            <span className="text-[12.5px] font-medium text-mid">
              Resources
              <span className="ml-1.5 font-normal text-faint">
                Monitors and servers that decide this component's status
              </span>
            </span>
            {mappings.length > 0 && (
              <ul className="divide-y divide-line-soft rounded-lg border border-line-soft">
                {mappings.map((mapping) => (
                  <li
                    key={mapping.id}
                    className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2 py-1.5 pr-1.5 pl-3 text-[13px]"
                  >
                    <span className="min-w-0">
                      <b className="block truncate font-medium">
                        {names.get(mapping.resource_id ?? "") ?? "Unknown resource"}
                      </b>
                      <small className="text-xs text-faint">
                        {KIND_LABEL[mapping.resource_type ?? ""] ?? mapping.resource_type}
                      </small>
                    </span>
                    <Button
                      size="sm"
                      variant="ghost"
                      aria-label="Remove resource"
                      onClick={() =>
                        removeMapping.mutate({
                          id: pageId,
                          componentId: component.id ?? "",
                          mappingId: mapping.id ?? "",
                        })
                      }
                    >
                      <XIcon className="size-3.5" />
                    </Button>
                  </li>
                ))}
              </ul>
            )}
            <div className="grid grid-cols-[110px_minmax(0,1fr)_auto] gap-2">
              <Select
                aria-label="Resource type"
                className="w-full"
                options={[
                  { value: "monitor", label: "Monitor" },
                  { value: "node", label: "Server" },
                ]}
                value={kind}
                onValueChange={(next) => {
                  setKind(next);
                  setResourceId("");
                }}
              />
              <Select
                aria-label="Resource"
                className="w-full"
                placeholder={kind === "monitor" ? "Choose a monitor" : "Choose a server"}
                options={options}
                value={resourceId}
                onValueChange={setResourceId}
              />
              <Button
                variant="secondary"
                disabled={!resourceId}
                onClick={() => {
                  addMapping.mutate({
                    id: pageId,
                    componentId: component.id ?? "",
                    data: { resource_type: kind, resource_id: resourceId },
                  });
                  setResourceId("");
                }}
              >
                <PlusIcon className="size-3.5" />
                Add
              </Button>
            </div>
          </div>
        )}
        <DialogFooter>
          {component?.id && (
            <Button
              variant="destructive"
              className="mr-auto"
              onClick={() => remove.mutate({ id: pageId, componentId: component.id ?? "" })}
            >
              Delete
            </Button>
          )}
          <Button disabled={!name.trim() || create.isPending || update.isPending} onClick={save}>
            {component ? "Save" : "Add component"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
