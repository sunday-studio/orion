import {
  Button,
  Checkbox,
  Input,
  Popover,
  PopoverContent,
  PopoverTrigger,
  Select,
  buttonStyles,
} from "@/design-system";

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
  const attentionFilterLabel =
    selectedAttentionFilters.length === 0
      ? "Attention filters"
      : `Attention: ${attentionFilterOptions
          .filter((option) => selectedAttentionFilters.includes(option.value))
          .map((option) => option.label)
          .join(", ")}`;

  const toggleAttention = (value: AttentionFilterValue, checked: boolean) => {
    onAttentionFiltersChange(
      checked
        ? [...selectedAttentionFilters, value]
        : selectedAttentionFilters.filter((selected) => selected !== value),
    );
  };

  return (
    <div className="space-y-2 py-2">
      <div className="flex flex-wrap items-center gap-2">
        <Input
          value={search}
          onChange={(event) => onSearchChange(event.target.value)}
          placeholder="Search servers or monitors"
          className="w-full max-w-sm sm:w-auto"
        />
        <Select
          label="Status"
          options={statusOptions}
          value={status}
          onValueChange={onStatusChange}
        />
        <Popover>
          <PopoverTrigger
            aria-label="Attention filters"
            className={buttonStyles({
              variant: "secondary",
              className: "max-w-64 justify-start truncate",
            })}
          >
            {attentionFilterLabel}
          </PopoverTrigger>
          <PopoverContent className="w-56 space-y-2">
            {attentionFilterOptions.map((option) => (
              <label key={option.value} className="flex items-center gap-2 text-[13px]">
                <Checkbox
                  checked={selectedAttentionFilters.includes(option.value)}
                  onCheckedChange={(checked) => toggleAttention(option.value, Boolean(checked))}
                />
                {option.label}
              </label>
            ))}
          </PopoverContent>
        </Popover>
        {hasFilters && (
          <Button variant="ghost" size="sm" onClick={onClear}>
            Clear
          </Button>
        )}
      </div>
    </div>
  );
};
