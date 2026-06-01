import { EmptyState } from "@/components/empty-state";
import { StatusBadge } from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import type { ApiStatusPageComponentResponse } from "@/orion-sdk";
import { RadioTower, Trash2 } from "lucide-react";
import { statusBadgeStatus } from "./status-pages-shared";

type StatusPageComponentsPanelProps = {
  components: ApiStatusPageComponentResponse[];
  deletePending: boolean;
  onRemoveComponent: (componentId?: string, label?: string) => void;
  onRemoveMapping: (componentId?: string, mappingId?: string) => void;
};

export const StatusPageComponentsPanel = ({
  components,
  deletePending,
  onRemoveComponent,
  onRemoveMapping,
}: StatusPageComponentsPanelProps) => (
  <div className="space-y-3">
    <h3 className="text-sm font-medium">Configured Components</h3>
    {components.length === 0 && (
      <EmptyState title="No components" description="Add a section and component." />
    )}
    {components.map((component) => (
      <div className="border border-neutral-200 p-3" key={component.id}>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <div className="font-medium">{component.public_name}</div>
            <div className="text-sm text-neutral-600">{component.public_description}</div>
          </div>
          <div className="flex items-center gap-2">
            <StatusBadge
              fallback={component.manual_status || component.display_mode}
              value={statusBadgeStatus(component.manual_status)}
            />
            <Button
              disabled={deletePending}
              onClick={() => onRemoveComponent(component.id, component.public_name)}
              size="sm"
              type="button"
              variant="outline"
            >
              <Trash2 className="size-3.5" />
              Remove
            </Button>
          </div>
        </div>
        <div className="mt-3 space-y-1 text-sm">
          {(component.mappings ?? []).map((mapping) => (
            <div className="flex flex-wrap items-center justify-between gap-2" key={mapping.id}>
              <div className="flex min-w-0 items-center gap-2">
                <RadioTower className="size-3.5 shrink-0 text-neutral-500" />
                <span>{mapping.resource_type}</span>
                <span className="min-w-0 truncate text-neutral-600">{mapping.resource_id}</span>
              </div>
              <Button
                disabled={deletePending}
                onClick={() => onRemoveMapping(component.id, mapping.id)}
                size="sm"
                type="button"
                variant="outline"
              >
                <Trash2 className="size-3.5" />
                Remove
              </Button>
            </div>
          ))}
          {(component.mappings ?? []).length === 0 && (
            <div className="text-neutral-600">No mappings</div>
          )}
        </div>
      </div>
    ))}
  </div>
);
