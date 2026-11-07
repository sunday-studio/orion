import { useEffect, useState } from "react";
import {
  Badge,
  Button,
  Checkbox,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Skeleton,
  useToast,
} from "@/design-system";
import { createService, linkServiceItem, useListServiceSuggestions } from "@/orion-sdk";
import type { ServiceServiceSuggestion } from "@/orion-sdk";
import { acceptPlan } from "../link-runner";
import {
  defaultSelectedSuggestions,
  memberKey,
  memberReason,
  planSuggestion,
  plural,
  suggestionKey,
  suggestionLikelihood,
} from "../services.domain";
import { useRefreshServices } from "../use-service-mutations";

const SuggestionGroup = ({
  suggestion,
  checked,
  skipped,
  onCheckedChange,
  onSkipMember,
}: {
  suggestion: ServiceServiceSuggestion;
  checked: boolean;
  skipped: ReadonlySet<string>;
  onCheckedChange: (checked: boolean) => void;
  onSkipMember: (key: string) => void;
}) => {
  const likely = suggestionLikelihood(suggestion) === "likely";
  return (
    <li className="grid grid-cols-[20px_minmax(0,1fr)_auto] items-start gap-2.5 rounded-lg border border-hairline p-3">
      <Checkbox
        aria-label={`Create ${suggestion.name}`}
        checked={checked}
        onCheckedChange={(next) => onCheckedChange(Boolean(next))}
        className="mt-0.5"
      />
      <div className="min-w-0">
        <b className="text-[13px]">{suggestion.name}</b>{" "}
        {suggestion.environment && <span className="text-faint">{suggestion.environment}</span>}
        {suggestion.service_id && (
          <span className="ml-1 text-xs text-quiet">adds to an existing service</span>
        )}
        <ul className="mt-2 grid gap-1.5">
          {(suggestion.members ?? []).map((member) => {
            const key = memberKey(member);
            const off = skipped.has(key);
            return (
              <li key={key} className="flex items-start gap-2 text-[12.5px]">
                <Checkbox
                  aria-label={`Include ${member.name}`}
                  checked={!off}
                  disabled={!checked}
                  onCheckedChange={() => onSkipMember(key)}
                  className="mt-0.5"
                />
                <span className="min-w-0">
                  <span className="rounded bg-tray px-1.5 py-0.5 text-xs">
                    {member.name} <span className="text-faint">{member.kind}</span>
                  </span>
                  <span className="ml-2 text-quiet">{memberReason(member)}</span>
                </span>
              </li>
            );
          })}
        </ul>
      </div>
      <Badge tone={likely ? "up" : "unknown"}>{likely ? "Likely" : "Maybe"}</Badge>
    </li>
  );
};

/** First run: group the apps, monitors and resources that look alike. Nothing links until accepted. */
export const SuggestionsDialog = ({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) => {
  const query = useListServiceSuggestions(undefined, { query: { enabled: open } });
  const suggestions = query.data?.suggestions ?? [];
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [skipped, setSkipped] = useState<ReadonlySet<string>>(new Set());
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const refresh = useRefreshServices();
  const toast = useToast();

  // Likely groups start ticked once, when the list first arrives.
  const loaded = query.isSuccess && open;
  useEffect(() => {
    if (loaded) setSelected(new Set(defaultSelectedSuggestions(query.data?.suggestions ?? [])));
  }, [loaded, query.data]);

  const chosen = suggestions.filter((s, i) => selected.has(suggestionKey(s, i)));
  const toggle = (key: string, set: ReadonlySet<string>, update: (next: Set<string>) => void) => {
    const next = new Set(set);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    update(next);
  };

  const accept = async () => {
    setPending(true);
    setError("");
    let failures = 0;
    let made = 0;
    for (const suggestion of chosen) {
      const result = await acceptPlan(planSuggestion(suggestion, skipped), {
        create: createService,
        link: linkServiceItem,
      });
      if (result.error) failures += 1;
      else made += 1;
      failures += result.outcome.failed.length;
    }
    setPending(false);
    refresh();
    if (made === 0 && failures > 0) {
      setError("Unable to create these services. Nothing was linked.");
      return;
    }
    toast.add({
      title: `Grouped ${plural(made, "service")}`,
      description: failures > 0 ? `${plural(failures, "link")} could not be added.` : undefined,
    });
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100vh-4rem)] max-w-[640px] gap-3.5 overflow-y-auto rounded-2xl p-5">
        <DialogHeader>
          <DialogTitle className="text-[16px]">Group what you already have</DialogTitle>
          <DialogDescription>
            {suggestions.length > 0
              ? `Orion found ${plural(suggestions.length, "possible service")}. Untick any that look wrong.`
              : "Orion looks at app names, monitor names and resources that match."}
          </DialogDescription>
        </DialogHeader>
        {query.isPending ? (
          <Skeleton className="h-24 w-full" />
        ) : query.isError ? (
          <p role="alert" className="text-[13px] text-[#be123c]">
            Unable to load suggestions.
          </p>
        ) : suggestions.length === 0 ? (
          <p className="text-[13px] text-quiet">
            Nothing to suggest. Everything that matches is already in a service.
          </p>
        ) : (
          <ul className="grid gap-2.5">
            {suggestions.map((suggestion, index) => {
              const key = suggestionKey(suggestion, index);
              return (
                <SuggestionGroup
                  key={key}
                  suggestion={suggestion}
                  checked={selected.has(key)}
                  skipped={skipped}
                  onCheckedChange={() => toggle(key, selected, setSelected)}
                  onSkipMember={(memberId) => toggle(memberId, skipped, setSkipped)}
                />
              );
            })}
          </ul>
        )}
        {error && (
          <p role="alert" className="text-[12.5px] text-[#be123c]">
            {error}
          </p>
        )}
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Skip for now
          </Button>
          <Button disabled={pending || chosen.length === 0} onClick={() => void accept()}>
            {pending ? "Creating..." : `Create ${plural(chosen.length, "service")}`}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
