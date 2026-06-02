import { Button } from "@/components/ui/button";
import {
  Combobox,
  ComboboxContent,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from "@/components/ui/combobox";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger } from "@/components/ui/select";

const attentionFilterOptions = [
  { value: "maintenance", label: "Maintenance" },
  { value: "stale", label: "Stale only" },
  { value: "incidents", label: "Has incidents" },
] as const;

const statusOptions = [
  { value: "all", label: "All statuses" },
  { value: "up", label: "Up" },
  { value: "down", label: "Down" },
  { value: "degraded", label: "Degraded" },
  { value: "maintenance", label: "Maintenance" },
  { value: "stale", label: "Stale" },
  { value: "unknown", label: "Unknown" },
] as const;

export type AttentionFilterValue = (typeof attentionFilterOptions)[number]["value"];

type AgentFiltersProps = {
  search: string;
  status: string;
  selectedAttentionFilters: AttentionFilterValue[];
  hasFilters: boolean;
  onSearchChange: (value: string) => void;
  onStatusChange: (value: string) => void;
  onAttentionFiltersChange: (values: AttentionFilterValue[]) => void;
  onClear: () => void;
};

export const AgentFilters = ({
  search,
  status,
  selectedAttentionFilters,
  hasFilters,
  onSearchChange,
  onStatusChange,
  onAttentionFiltersChange,
  onClear,
}: AgentFiltersProps) => {
  const statusLabel = statusOptions.find((option) => option.value === status)?.label ?? status;
  const attentionFilterLabel =
    selectedAttentionFilters.length === 0
      ? "Attention filters"
      : `Attention: ${attentionFilterOptions
          .filter((option) => selectedAttentionFilters.includes(option.value))
          .map((option) => option.label)
          .join(", ")}`;

  return (
    <div className="space-y-2 py-2">
      <div className="flex flex-wrap items-center gap-2">
        <Input
          value={search}
          onChange={(event) => onSearchChange(event.target.value)}
          placeholder="Search servers or monitors"
          className="w-full max-w-sm sm:w-auto"
        />
        <Select value={status} onValueChange={onStatusChange}>
          <SelectTrigger className="w-48 rounded-full text-xs">
            <span data-slot="select-value">Status: {statusLabel}</span>
          </SelectTrigger>
          <SelectContent>
            {statusOptions.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Combobox
          multiple
          value={selectedAttentionFilters}
          onValueChange={(values) => onAttentionFiltersChange(values)}
        >
          <ComboboxInput
            readOnly
            value={attentionFilterLabel}
            className="w-full sm:w-56"
            aria-label="Attention filters"
          />
          <ComboboxContent>
            <ComboboxList>
              {attentionFilterOptions.map((option) => (
                <ComboboxItem key={option.value} value={option.value}>
                  {option.label}
                </ComboboxItem>
              ))}
            </ComboboxList>
          </ComboboxContent>
        </Combobox>
        {hasFilters && (
          <Button variant="ghost" size="sm" onClick={onClear}>
            Clear
          </Button>
        )}
      </div>
    </div>
  );
};
