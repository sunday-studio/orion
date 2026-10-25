import { useEffect, useState } from "react";
import {
  Button,
  Callout,
  FieldPanel,
  GridRow,
  GridTable,
  Input,
  RaisedCard,
  Select,
  Textarea,
  useToast,
} from "@/design-system";
import {
  type ApiApp,
  type ApiRedactionRule,
  useGetRedactionDefaults,
  useTestAppRedaction,
  useUpdateApp,
} from "@/orion-sdk";
import { ChipInput } from "./chip-input";

const COLUMNS = "90px minmax(0,1fr) minmax(0,1fr) 90px";
const SAMPLE = '{"user":"u_41","card_number":"4242424242424242","token":"abc123"}';

export const RedactionTab = ({ app, onSaved }: { app: ApiApp; onSaved: () => void }) => {
  const toast = useToast();
  const defaults = useGetRedactionDefaults();
  const saved = app.redaction;
  const [extra, setExtra] = useState(saved?.extra_keys ?? []);
  const [rules, setRules] = useState<ApiRedactionRule[]>(saved?.rules ?? []);
  const [sample, setSample] = useState(SAMPLE);
  const [output, setOutput] = useState("");
  const [draft, setDraft] = useState<ApiRedactionRule>();
  const id = app.id ?? "";
  const dirty =
    JSON.stringify(extra) !== JSON.stringify(saved?.extra_keys ?? []) ||
    JSON.stringify(rules) !== JSON.stringify(saved?.rules ?? []);
  const defaultKeys = saved?.default_keys ?? defaults.data?.keys ?? [];
  const test = useTestAppRedaction({
    mutation: {
      onSuccess: (result) => setOutput(JSON.stringify(result.redacted)),
      onError: () => setOutput(""),
    },
  });
  const save = useUpdateApp({
    mutation: {
      onSuccess: () => {
        onSaved();
        toast.add({ title: "Rules saved" });
      },
      onError: (failure) => toast.add({ title: (failure as Error).message || "Unable to save" }),
    },
  });

  useEffect(() => {
    const timer = window.setTimeout(() => {
      let parsed: unknown = sample;
      try {
        parsed = JSON.parse(sample);
      } catch {
        // A plain string is a valid sample too.
      }
      test.mutate({
        id,
        data: {
          sample: parsed as Record<string, unknown>,
          redaction: { extra_keys: extra, rules },
        },
      });
    }, 500);
    return () => window.clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sample, extra, rules, id]);

  return (
    <div className="flex flex-col gap-3.5">
      <Callout tone="up" lead="Redaction is on.">
        It runs when a log arrives. Redacted values cannot be recovered.
      </Callout>
      <FieldPanel title="Default keys" aside="Matched case-insensitively at any depth">
        <ChipInput label="Redacted keys" chips={extra} fixed={defaultKeys} onChange={setExtra} />
      </FieldPanel>
      <FieldPanel title="Custom rules" aside="Applied after the defaults">
        {(rules.length > 0 || draft) && (
          <RaisedCard>
            <GridTable>
              <GridRow columns={COLUMNS} header>
                <span>Type</span>
                <span>Pattern</span>
                <span>Replace with</span>
                <span />
              </GridRow>
              {rules.map((rule, index) => (
                <GridRow key={`${rule.pattern}-${index}`} columns={COLUMNS}>
                  <span className="text-faint capitalize">{rule.type}</span>
                  <span className="truncate font-mono text-xs text-faint">{rule.pattern}</span>
                  <span className="text-faint">{rule.replace_with}</span>
                  <span className="text-right">
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => setRules(rules.filter((_, i) => i !== index))}
                    >
                      Remove
                    </Button>
                  </span>
                </GridRow>
              ))}
              {draft && (
                <GridRow columns={COLUMNS} className="h-auto py-2">
                  <Select
                    aria-label="Rule type"
                    options={[
                      { value: "key", label: "Key" },
                      { value: "regex", label: "Regex" },
                    ]}
                    value={draft.type ?? "key"}
                    onValueChange={(type) => setDraft({ ...draft, type: type as "key" | "regex" })}
                  />
                  <Input
                    aria-label="Pattern"
                    placeholder="card_number"
                    value={draft.pattern ?? ""}
                    onChange={(e) => setDraft({ ...draft, pattern: e.target.value })}
                  />
                  <Input
                    aria-label="Replace with"
                    placeholder="[redacted]"
                    value={draft.replace_with ?? ""}
                    onChange={(e) => setDraft({ ...draft, replace_with: e.target.value })}
                  />
                  <Button
                    size="sm"
                    disabled={!draft.pattern?.trim()}
                    onClick={() => {
                      setRules([
                        ...rules,
                        { ...draft, replace_with: draft.replace_with || "[redacted]" },
                      ]);
                      setDraft(undefined);
                    }}
                  >
                    Add
                  </Button>
                </GridRow>
              )}
            </GridTable>
          </RaisedCard>
        )}
        {!draft && (
          <Button
            size="sm"
            variant="secondary"
            className="mt-3"
            disabled={rules.length >= 20}
            onClick={() => setDraft({ type: "key", pattern: "", replace_with: "[redacted]" })}
          >
            + Add rule
          </Button>
        )}
      </FieldPanel>
      <FieldPanel title="Test your rules" aside="Nothing is saved">
        <div className="grid gap-2">
          <Textarea
            aria-label="Sample log"
            rows={3}
            className="font-mono text-[12.5px]"
            value={sample}
            onChange={(event) => setSample(event.target.value)}
          />
          <pre className="m-0 min-h-10 overflow-x-auto rounded-lg bg-[#142a23] px-3 py-3 font-mono text-[12.5px] text-[#e6f1ec]">
            {test.isError ? "Core could not apply these rules." : output || "..."}
          </pre>
        </div>
      </FieldPanel>
      <div className="flex justify-end">
        <Button
          disabled={save.isPending || !dirty}
          onClick={() => save.mutate({ id, data: { redaction: { extra_keys: extra, rules } } })}
        >
          Save rules
        </Button>
      </div>
    </div>
  );
};
