import { useState } from "react";
import { Button, EmptyState, Input, useToast } from "@/design-system";
import {
  type ApiStatusPageDetailResponse,
  updateStatusPageComponent,
  useCreateStatusPageSection,
  usePreviewStatusPage,
} from "@/orion-sdk";
import { ComponentDialog, NO_SECTION } from "./component-dialog";
import { componentPayload } from "./component-payload";
import { ComponentsBulkBar } from "./components-bulk-bar";
import { ComponentsGroup } from "./components-group";

const ORDER_STEP = 10;

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
  // Bumped when a reorder fails so each group drops its dragged order.
  const [orderEpoch, setOrderEpoch] = useState(0);
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
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
  const chosen = components.filter((c) => selected.has(c.id ?? ""));

  const toggle = (id: string, checked: boolean) =>
    setSelected((current) => {
      const next = new Set(current);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  const toggleAll = (ids: readonly string[], checked: boolean) =>
    setSelected((current) => {
      const next = new Set(current);
      for (const id of ids) {
        if (checked) next.add(id);
        else next.delete(id);
      }
      return next;
    });

  const reorder = async (ids: readonly string[]) => {
    const byId = new Map(components.map((c) => [c.id ?? "", c]));
    const results = await Promise.allSettled(
      ids.flatMap((id, index) => {
        const component = byId.get(id);
        if (!component || component.sort_order === index * ORDER_STEP) return [];
        return updateStatusPageComponent(
          pageId,
          id,
          componentPayload(component, { sort_order: index * ORDER_STEP }),
        );
      }),
    );
    if (results.some((result) => result.status === "rejected")) {
      toast.add({ title: "Unable to save the new order" });
      setOrderEpoch((epoch) => epoch + 1);
    }
    onChanged();
  };

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
            <ComponentsGroup
              key={`${group.id}:${orderEpoch}`}
              name={group.name}
              items={group.items}
              status={status}
              selected={selected}
              onToggle={toggle}
              onToggleAll={toggleAll}
              onEdit={setEditing}
              onReorder={(ids) => void reorder(ids)}
            />
          ))
      )}
      {chosen.length > 0 && (
        <ComponentsBulkBar
          pageId={pageId}
          detail={detail}
          selected={chosen}
          onClear={() => setSelected(new Set())}
          onChanged={onChanged}
        />
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
