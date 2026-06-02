import { Button } from "@/components/ui/button";
import type { ApiStatusPageIncidentComponentSuggestionResponse } from "@/orion-sdk";
import { suggestionMatchLabel } from "./status-pages-shared";

type IncidentSuggestionsProps = {
  internalIncidentId: string;
  isError: boolean;
  isLoading: boolean;
  onApply: () => void;
  suggestions: ApiStatusPageIncidentComponentSuggestionResponse[];
};

export const IncidentSuggestions = ({
  internalIncidentId,
  isError,
  isLoading,
  onApply,
  suggestions,
}: IncidentSuggestionsProps) => {
  if (!internalIncidentId) return null;

  return (
    <div className="space-y-2 border border-neutral-200 p-3 text-sm">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="font-medium">Suggested components</div>
        <Button
          disabled={isLoading || suggestions.length === 0}
          onClick={onApply}
          type="button"
          variant="outline"
        >
          Apply suggestions
        </Button>
      </div>
      {isLoading && <div className="text-neutral-600">Loading suggested components...</div>}
      {isError && <div>Unable to load suggested components.</div>}
      {!isLoading && !isError && suggestions.length === 0 && (
        <div className="text-neutral-600">No mapped public components found.</div>
      )}
      {suggestions.length > 0 && (
        <div className="space-y-1">
          {suggestions.map((suggestion) => (
            <div
              className="flex flex-wrap items-center justify-between gap-2"
              key={suggestion.component_id}
            >
              <span>{suggestion.component_name}</span>
              <span className="text-neutral-600">{suggestionMatchLabel(suggestion)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
