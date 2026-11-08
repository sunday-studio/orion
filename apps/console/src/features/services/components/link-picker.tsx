import { useState } from "react";
import type { ServiceServiceLinkRef } from "@/orion-sdk";
import { Checkbox, Input, Select, Skeleton } from "@/design-system";
import {
  useGetMonitors,
  useGetNodeAppResources,
  useGetNodes,
  useListApps,
  useListServices,
} from "@/orion-sdk";
import {
  type LinkCandidate,
  appCandidates,
  elsewhereNote,
  monitorCandidates,
  refsForKeys,
  resourceCandidates,
} from "../services.domain";

export type PickerKind = "monitor" | "app" | "resource";

type Props = {
  kind: PickerKind;
  /** The service being edited; its own links are left out of the list. */
  serviceId?: string;
  /** Called with the ticked things, as link requests, each time a tick changes. */
  onChange: (refs: ServiceServiceLinkRef[]) => void;
};

const List = ({
  candidates,
  loading,
  onChange,
  noun,
}: {
  candidates: LinkCandidate[];
  loading: boolean;
  onChange: (refs: ServiceServiceLinkRef[]) => void;
  noun: string;
}) => {
  const [filter, setFilter] = useState("");
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const toggle = (candidate: LinkCandidate) => {
    const next = new Set(selected);
    if (next.has(candidate.key)) next.delete(candidate.key);
    else next.add(candidate.key);
    setSelected(next);
    onChange(refsForKeys(candidates, next));
  };
  const services = useListServices().data?.services ?? [];
  const shown = candidates.filter((candidate) =>
    candidate.label.toLowerCase().includes(filter.trim().toLowerCase()),
  );
  if (loading) return <Skeleton className="h-20 w-full" />;
  return (
    <div className="grid gap-2">
      {candidates.length > 6 && (
        <Input
          aria-label={`Filter ${noun}`}
          placeholder={`Filter ${noun}`}
          value={filter}
          onChange={(event) => setFilter(event.target.value)}
        />
      )}
      {shown.length === 0 ? (
        <p className="py-2 text-[12.5px] text-quiet">Nothing left to link.</p>
      ) : (
        <ul className="max-h-48 overflow-y-auto rounded-lg border border-hairline">
          {shown.map((candidate) => {
            const note = elsewhereNote(candidate, services);
            return (
              <li key={candidate.key} className="border-t border-line-soft first:border-t-0">
                <label className="flex cursor-pointer items-center gap-2.5 px-3 py-2 text-[13px] hover:bg-sub">
                  <Checkbox
                    aria-label={`Link ${candidate.label}`}
                    checked={selected.has(candidate.key)}
                    onCheckedChange={() => toggle(candidate)}
                  />
                  <span className="min-w-0 flex-1 truncate">
                    <b className="font-medium">{candidate.label}</b>{" "}
                    {candidate.hint && <span className="text-faint">{candidate.hint}</span>}
                  </span>
                  {note && <span className="shrink-0 text-xs text-[#b45309]">{note}</span>}
                </label>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
};

const MonitorPicker = ({ serviceId, onChange }: Omit<Props, "kind">) => {
  const query = useGetMonitors({ limit: 200 });
  const candidates = monitorCandidates(query.data?.monitors ?? [], serviceId);
  return (
    <List candidates={candidates} loading={query.isPending} onChange={onChange} noun="monitors" />
  );
};

const AppPicker = ({ serviceId, onChange }: Omit<Props, "kind">) => {
  const query = useListApps();
  const candidates = appCandidates(query.data?.apps ?? [], serviceId);
  return <List candidates={candidates} loading={query.isPending} onChange={onChange} noun="apps" />;
};

const ResourcePicker = ({ serviceId, onChange }: Omit<Props, "kind">) => {
  const nodes = useGetNodes({ limit: 200 });
  const [nodeId, setNodeId] = useState("");
  const options = (nodes.data?.nodes ?? []).map((node) => ({
    value: node.id ?? "",
    label: node.name ?? node.id ?? "Server",
  }));
  const activeId = nodeId || options[0]?.value || "";
  const resources = useGetNodeAppResources(activeId, { query: { enabled: Boolean(activeId) } });
  const node = { id: activeId, name: options.find((o) => o.value === activeId)?.label };
  const candidates = resourceCandidates(
    resources.data?.app_resources?.series ?? [],
    node,
    serviceId,
  );
  return (
    <div className="grid gap-2">
      {options.length > 1 && (
        <Select
          aria-label="Server"
          label="Server"
          className="w-full"
          options={options}
          value={activeId}
          onValueChange={setNodeId}
        />
      )}
      {options.length === 0 && !nodes.isPending ? (
        <p className="py-2 text-[12.5px] text-quiet">No servers report resources yet.</p>
      ) : (
        <List
          key={activeId}
          candidates={candidates}
          loading={nodes.isPending || resources.isPending}
          onChange={onChange}
          noun="resources"
        />
      )}
    </div>
  );
};

/** Checkbox list of things that can be linked. Picking never links on its own. */
export const LinkPicker = ({ kind, ...props }: Props) =>
  kind === "monitor" ? (
    <MonitorPicker {...props} />
  ) : kind === "app" ? (
    <AppPicker {...props} />
  ) : (
    <ResourcePicker {...props} />
  );
