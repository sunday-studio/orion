import { useState } from "react";
import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  FieldPanel,
  Input,
  RaisedCard,
  Select,
  StatusBadge,
  useToast,
} from "@/design-system";
import {
  type ApiStatusPageComponentResponse,
  type ApiStatusPageDetailResponse,
  useCreateStatusPageComponent,
  useCreateStatusPageComponentMapping,
  useCreateStatusPageSection,
  useDeleteStatusPageComponent,
  useDeleteStatusPageComponentMapping,
  useGetAgents,
  useGetMonitors,
  usePreviewStatusPage,
  useUpdateStatusPageComponent,
} from "@/orion-sdk";
import { publicStatusLabel, publicStatusTone } from "../status-format";

const NO_SECTION = "";

const ComponentDialog = ({
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
  const agents = useGetAgents({ limit: 200 }, { query: { enabled: open } });
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
  const options =
    kind === "monitor"
      ? (monitors.data?.monitors ?? []).map((m) => ({ value: m.id ?? "", label: m.name ?? "" }))
      : (agents.data?.agents ?? []).map((a) => ({ value: a.id ?? "", label: a.name ?? "" }));
  const mappings = component?.mappings ?? [];
  const save = () => {
    const data = {
      display_mode: component?.display_mode,
      manual_status: component?.manual_status,
      manual_status_reason: component?.manual_status_reason,
      public_description: component?.public_description,
      sort_order: component?.sort_order,
      public_name: name.trim(),
      section_id: sectionId || undefined,
      visible: component?.visible ?? true,
    };
    if (component?.id) update.mutate({ id: pageId, componentId: component.id, data });
    else create.mutate({ id: pageId, data });
  };
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[480px] gap-3.5 rounded-2xl p-5">
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
            <span className="text-[12.5px] font-medium text-mid">Resources</span>
            {mappings.map((mapping) => (
              <div
                key={mapping.id}
                className="flex items-center justify-between rounded-lg border border-line-soft px-3 py-2 text-[13px]"
              >
                <span className="truncate">
                  {mapping.resource_type} · {mapping.resource_id}
                </span>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() =>
                    removeMapping.mutate({
                      id: pageId,
                      componentId: component.id ?? "",
                      mappingId: mapping.id ?? "",
                    })
                  }
                >
                  Remove
                </Button>
              </div>
            ))}
            <div className="flex gap-2">
              <Select
                aria-label="Resource type"
                options={[
                  { value: "monitor", label: "Monitor" },
                  { value: "agent", label: "Server" },
                ]}
                value={kind}
                onValueChange={(next) => {
                  setKind(next);
                  setResourceId("");
                }}
              />
              <Select
                aria-label="Resource"
                className="min-w-0 flex-1"
                options={options}
                value={resourceId}
                onValueChange={setResourceId}
              />
              <Button
                size="sm"
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
                Add
              </Button>
            </div>
          </div>
        )}
        <DialogFooter>
          {component?.id && (
            <Button
              variant="destructive"
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

export const ComponentsTab = ({
  pageId,
  detail,
  onChanged,
}: {
  pageId: string;
  detail?: ApiStatusPageDetailResponse;
  onChanged: () => void;
}) => {
  const toast = useToast();
  const preview = usePreviewStatusPage(pageId);
  const [editing, setEditing] = useState<string>();
  const [sectionName, setSectionName] = useState("");
  const addSection = useCreateStatusPageSection({
    mutation: {
      onSuccess: () => {
        setSectionName("");
        onChanged();
      },
      onError: () => toast.add({ title: "Unable to add the section" }),
    },
  });
  const status = new Map<string, string | undefined>();
  for (const section of preview.data?.preview?.sections ?? [])
    for (const item of section.components ?? []) if (item.id) status.set(item.id, item.status);

  const components = detail?.components ?? [];
  const groups = [
    ...(detail?.sections ?? []).map((s) => ({ id: s.id ?? "", name: s.name ?? "Section" })),
    { id: NO_SECTION, name: "Other" },
  ].map((group) => ({
    ...group,
    items: components.filter((c) => (c.section_id ?? NO_SECTION) === group.id),
  }));

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-2">
        <Input
          aria-label="New section name"
          placeholder="Section name"
          className="w-[220px]"
          value={sectionName}
          onChange={(event) => setSectionName(event.target.value)}
        />
        <Button
          size="sm"
          variant="secondary"
          disabled={!sectionName.trim()}
          onClick={() => addSection.mutate({ id: pageId, data: { name: sectionName.trim() } })}
        >
          Add section
        </Button>
        <span className="flex-1" />
        <Button size="sm" onClick={() => setEditing("new")}>
          Add component
        </Button>
      </div>
      {components.length === 0 && (detail?.sections ?? []).length === 0 ? (
        <EmptyState
          title="No components yet"
          description="Components are the services visitors see on your page."
        />
      ) : (
        groups
          .filter((group) => group.items.length > 0 || group.id !== NO_SECTION)
          .map((group) => (
            <FieldPanel key={group.id} title={group.name} aside={`${group.items.length}`}>
              <RaisedCard>
                {group.items.map((component) => (
                  <div
                    key={component.id}
                    className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3 border-t border-line-soft px-3.5 py-2.5 text-[13px] first:border-t-0"
                  >
                    <span className="min-w-0">
                      <b className="block truncate font-medium">{component.public_name}</b>
                      <small className="text-xs text-faint">
                        {(component.mappings ?? []).length} resources
                      </small>
                    </span>
                    <StatusBadge status={publicStatusTone(status.get(component.id ?? ""))}>
                      {publicStatusLabel(status.get(component.id ?? ""))}
                    </StatusBadge>
                    <Button size="sm" variant="secondary" onClick={() => setEditing(component.id)}>
                      Edit
                    </Button>
                  </div>
                ))}
              </RaisedCard>
            </FieldPanel>
          ))
      )}
      {editing && (
        <ComponentDialog
          key={editing}
          pageId={pageId}
          detail={detail}
          component={components.find((item) => item.id === editing)}
          open
          onOpenChange={(open) => !open && setEditing(undefined)}
          onChanged={onChanged}
        />
      )}
    </div>
  );
};
