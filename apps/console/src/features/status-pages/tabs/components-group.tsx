import { GripVerticalIcon } from "lucide-react";
import { Reorder, useDragControls } from "motion/react";
import { useEffect, useRef, useState } from "react";
import { Button, Checkbox, FieldPanel, RaisedCard, StatusBadge } from "@/design-system";
import type { ApiStatusPageComponentResponse } from "@/orion-sdk";
import { publicStatusLabel, publicStatusTone } from "../status-format";

const Row = ({
  component,
  status,
  checked,
  onCheckedChange,
  onEdit,
  onDrop,
}: {
  component: ApiStatusPageComponentResponse;
  status?: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  onEdit: () => void;
  onDrop: () => void;
}) => {
  const controls = useDragControls();
  const resources = (component.mappings ?? []).length;
  return (
    <Reorder.Item
      as="div"
      value={component.id ?? ""}
      dragListener={false}
      dragControls={controls}
      onDragEnd={onDrop}
      className="relative grid grid-cols-[auto_auto_minmax(0,1fr)_auto_auto] items-center gap-3 border-t border-line-soft bg-card px-3.5 py-2.5 text-[13px] first:border-t-0"
    >
      <button
        type="button"
        aria-label={`Reorder ${component.public_name}`}
        className="-mx-1 cursor-grab touch-none rounded p-0.5 text-faint hover:text-ink active:cursor-grabbing"
        onPointerDown={(event) => controls.start(event)}
      >
        <GripVerticalIcon className="size-4" />
      </button>
      <Checkbox
        aria-label={`Select ${component.public_name}`}
        checked={checked}
        onCheckedChange={onCheckedChange}
      />
      <span className="min-w-0">
        <b className="block truncate font-medium">{component.public_name}</b>
        <small className="text-xs text-faint">
          {resources} {resources === 1 ? "resource" : "resources"}
          {component.visible === false && " · Hidden"}
        </small>
      </span>
      <StatusBadge status={publicStatusTone(status)}>{publicStatusLabel(status)}</StatusBadge>
      <Button size="sm" variant="secondary" onClick={onEdit}>
        Edit
      </Button>
    </Reorder.Item>
  );
};

export const ComponentsGroup = ({
  name,
  items,
  status,
  selected,
  onToggle,
  onToggleAll,
  onEdit,
  onReorder,
}: {
  name: string;
  items: readonly ApiStatusPageComponentResponse[];
  status: Map<string, string | undefined>;
  selected: ReadonlySet<string>;
  onToggle: (id: string, checked: boolean) => void;
  onToggleAll: (ids: readonly string[], checked: boolean) => void;
  onEdit: (id: string) => void;
  /** Called with the component ids in their new order once a drag ends. */
  onReorder: (ids: readonly string[]) => void;
}) => {
  const serverOrder = items.map((item) => item.id ?? "");
  const [order, setOrder] = useState(serverOrder);
  const latest = useRef(order);
  const key = serverOrder.join("|");
  useEffect(() => {
    setOrder(key ? key.split("|") : []);
  }, [key]);
  latest.current = order;
  const byId = new Map(items.map((item) => [item.id ?? "", item]));
  const checkedCount = serverOrder.filter((id) => selected.has(id)).length;

  return (
    <FieldPanel
      title={name}
      aside={
        items.length > 0 ? (
          <span className="flex items-center gap-2">
            <Checkbox
              aria-label={`Select all in ${name}`}
              checked={checkedCount === items.length}
              indeterminate={checkedCount > 0 && checkedCount < items.length}
              onCheckedChange={(checked) => onToggleAll(serverOrder, checked)}
            />
            {items.length}
          </span>
        ) : (
          "0"
        )
      }
    >
      <RaisedCard>
        <Reorder.Group as="div" axis="y" values={order} onReorder={setOrder}>
          {order.map((id) => {
            const component = byId.get(id);
            return component ? (
              <Row
                key={id}
                component={component}
                status={status.get(id)}
                checked={selected.has(id)}
                onCheckedChange={(checked) => onToggle(id, checked)}
                onEdit={() => onEdit(id)}
                onDrop={() => {
                  if (latest.current.join("|") !== key) onReorder(latest.current);
                }}
              />
            ) : null;
          })}
        </Reorder.Group>
        {items.length === 0 && (
          <p className="px-3.5 py-3 text-[13px] text-faint">No components in this section.</p>
        )}
      </RaisedCard>
    </FieldPanel>
  );
};
