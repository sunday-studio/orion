import { useState } from "react";
import {
  Button,
  FieldPanel,
  GridRow,
  GridTable,
  RaisedCard,
  Select,
  useToast,
} from "@/design-system";
import { LevelBadge } from "@/features/logs/components/log-states";
import { type ApiApp, useGetLogFieldCatalog, useSearchLogs, useUpdateApp } from "@/orion-sdk";
import { formatDate } from "@/utils/date";
import { ChipInput } from "./chip-input";

const COLUMNS = "100px 90px minmax(0,1fr) minmax(0,1.4fr)";
const TITLES = [{ value: "", label: "event, then the first line of the message" }];

const lookup = (attributes: unknown, path: string): string => {
  let value: unknown = attributes;
  for (const part of path.split(".")) {
    if (value && typeof value === "object") value = (value as Record<string, unknown>)[part];
    else return "-";
  }
  return value === undefined || value === null ? "-" : String(value);
};

export const DisplayTab = ({ app, onSaved }: { app: ApiApp; onSaved: () => void }) => {
  const toast = useToast();
  const profile = app.display_profile;
  const [title, setTitle] = useState(profile?.title_field ?? "");
  const [columns, setColumns] = useState(profile?.columns ?? []);
  const [hidden, setHidden] = useState(profile?.hidden ?? []);
  const id = app.id ?? "";
  const catalog = useGetLogFieldCatalog({ app_id: id });
  const sample = useSearchLogs({ app_id: id, limit: 2 });
  const sameList = (a: string[], b: string[]) =>
    a.length === b.length && a.every((x, i) => x === b[i]);
  const dirty =
    title !== (profile?.title_field ?? "") ||
    !sameList(columns, profile?.columns ?? []) ||
    !sameList(hidden, profile?.hidden ?? []);
  const fields = (catalog.data?.fields ?? []).map((field) => field.key ?? "").filter(Boolean);
  const save = useUpdateApp({
    mutation: {
      onSuccess: () => {
        onSaved();
        toast.add({ title: "Profile saved" });
      },
      onError: (failure) => toast.add({ title: (failure as Error).message || "Unable to save" }),
    },
  });
  return (
    <div className="flex flex-col gap-3.5">
      <p className="text-[13px] text-quiet">
        Optional. By default Orion picks sensible columns. A profile only changes how this app looks
        in Logs.
      </p>
      <FieldPanel title="Title" aside="The bold part of each row">
        <div className="grid gap-1.5">
          <span className="text-[12.5px] font-medium">Use</span>
          <Select
            aria-label="Title field"
            className="w-full"
            options={[...TITLES, ...fields.map((value) => ({ value, label: value }))]}
            value={title}
            onValueChange={setTitle}
          />
        </div>
      </FieldPanel>
      <FieldPanel title="Columns" aside="From the field catalog">
        <ChipInput label="Columns" chips={columns} suggestions={fields} onChange={setColumns} />
      </FieldPanel>
      <FieldPanel title="Hidden fields" aside="Never shown in lists or the drawer">
        <ChipInput label="Hidden fields" chips={hidden} suggestions={fields} onChange={setHidden} />
      </FieldPanel>
      <FieldPanel title="Preview" aside="How two recent lines will look">
        <RaisedCard>
          <GridTable>
            <GridRow columns={COLUMNS} header>
              <span>Time</span>
              <span>Severity</span>
              <span>Title</span>
              <span>Columns</span>
            </GridRow>
            {(sample.data?.entries ?? []).map((entry) => (
              <GridRow key={entry.id} columns={COLUMNS}>
                <span className="text-faint">{formatDate(entry.timestamp, "HH:mm:ss")}</span>
                <LevelBadge level={entry.severity} />
                <b className="font-medium">
                  {title
                    ? lookup(entry.attributes, title)
                    : (entry.event ?? entry.body?.split("\n")[0])}
                </b>
                <span className="text-faint">
                  {columns
                    .map((column) => `${column}=${lookup(entry.attributes, column)}`)
                    .join(", ")}
                </span>
              </GridRow>
            ))}
          </GridTable>
        </RaisedCard>
      </FieldPanel>
      <div className="flex justify-end">
        <Button
          disabled={save.isPending || !dirty}
          onClick={() =>
            save.mutate({
              id,
              data: { display_profile: { title_field: title, columns, hidden } },
            })
          }
        >
          Save profile
        </Button>
      </div>
    </div>
  );
};
