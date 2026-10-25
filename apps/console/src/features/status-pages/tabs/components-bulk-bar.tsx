import { useState } from "react";
import { Button, ConfirmDialog, FloatingBar, Select, useToast } from "@/design-system";
import {
  type ApiStatusPageComponentResponse,
  type ApiStatusPageDetailResponse,
  deleteStatusPageComponent,
  updateStatusPageComponent,
} from "@/orion-sdk";
import { componentPayload } from "./component-payload";

const MOVE_PLACEHOLDER = "move";

export const ComponentsBulkBar = ({
  pageId,
  detail,
  selected,
  onClear,
  onChanged,
}: {
  pageId: string;
  detail?: ApiStatusPageDetailResponse;
  selected: readonly ApiStatusPageComponentResponse[];
  onClear: () => void;
  onChanged: () => void;
}) => {
  const toast = useToast();
  const [busy, setBusy] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const run = async (
    verb: string,
    task: (component: ApiStatusPageComponentResponse) => Promise<unknown>,
  ) => {
    setBusy(true);
    const results = await Promise.allSettled(selected.map(task));
    const failed = results.filter((result) => result.status === "rejected").length;
    setBusy(false);
    setConfirmDelete(false);
    onChanged();
    if (failed === 0) {
      onClear();
      toast.add({
        title: `${selected.length} ${selected.length === 1 ? "component" : "components"} ${verb}`,
      });
    } else toast.add({ title: `${failed} of ${selected.length} could not be ${verb}` });
  };

  const setVisible = (visible: boolean) =>
    run(visible ? "shown" : "hidden", (component) =>
      updateStatusPageComponent(
        pageId,
        component.id ?? "",
        componentPayload(component, { visible }),
      ),
    );
  const moveTo = (sectionId: string) =>
    run("moved", (component) =>
      updateStatusPageComponent(
        pageId,
        component.id ?? "",
        componentPayload(component, { section_id: sectionId }),
      ),
    );

  return (
    <>
      <FloatingBar aria-label="Bulk actions">
        <span className="pr-1 font-medium">{selected.length} selected</span>
        <span aria-hidden className="mx-1 h-5 w-px bg-hairline" />
        <Button size="sm" variant="secondary" disabled={busy} onClick={() => void setVisible(true)}>
          Show
        </Button>
        <Button
          size="sm"
          variant="secondary"
          disabled={busy}
          onClick={() => void setVisible(false)}
        >
          Hide
        </Button>
        <Select
          aria-label="Move to section"
          label="Move to"
          className="h-6 min-w-[150px] text-xs"
          disabled={busy}
          placeholder="Section"
          options={(detail?.sections ?? []).map((s) => ({
            value: s.id ?? "",
            label: s.name ?? "",
          }))}
          // Matches no option, so the trigger keeps showing the placeholder after a pick.
          value={MOVE_PLACEHOLDER}
          onValueChange={(value) => void moveTo(value)}
        />
        <Button
          size="sm"
          variant="destructive"
          disabled={busy}
          onClick={() => setConfirmDelete(true)}
        >
          Delete
        </Button>
        <Button size="sm" variant="ghost" onClick={onClear}>
          Clear
        </Button>
      </FloatingBar>
      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        title={`Delete ${selected.length} ${selected.length === 1 ? "component" : "components"}?`}
        description="They disappear from the status page. The monitors and servers behind them stay."
        confirmLabel="Delete"
        onConfirm={() =>
          void run("deleted", (component) => deleteStatusPageComponent(pageId, component.id ?? ""))
        }
      />
    </>
  );
};
