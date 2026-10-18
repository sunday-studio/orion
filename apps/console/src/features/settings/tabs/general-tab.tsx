import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { Button, Callout, FieldPanel, Input, Select, Skeleton, useToast } from "@/design-system";
import {
  getGetGeneralSettingsQueryKey,
  useGetGeneralSettings,
  useUpdateGeneralSettings,
} from "@/orion-sdk";
import { setTimezone } from "@/utils/date";
import { Labeled } from "../components-ui";

const zones = () => {
  try {
    return [
      "UTC",
      ...(Intl as unknown as { supportedValuesOf: (k: string) => string[] }).supportedValuesOf(
        "timeZone",
      ),
    ];
  } catch {
    return ["UTC"];
  }
};

export const GeneralTab = () => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const query = useGetGeneralSettings();
  const data = query.data;
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [zone, setZone] = useState("UTC");
  const [format, setFormat] = useState("24");
  const options = useMemo(
    () =>
      [...new Set(zones())].map((value) => ({
        value,
        label: value === "UTC" ? "UTC (default)" : value,
      })),
    [],
  );

  useEffect(() => {
    if (!data) return;
    setName(data.instance_name ?? "");
    setUrl(data.public_url ?? "");
    setZone(data.timezone || "UTC");
    setFormat(data.time_format ?? "24");
  }, [data]);

  const save = useUpdateGeneralSettings({
    mutation: {
      onSuccess: () => {
        setTimezone(zone);
        void queryClient.invalidateQueries({ queryKey: getGetGeneralSettingsQueryKey() });
        toast.add({ title: "Settings saved" });
      },
      onError: (failure) =>
        toast.add({ title: (failure as Error).message || "Unable to save settings" }),
    },
  });

  if (query.isPending) return <Skeleton className="h-64 w-full rounded-xl" />;
  if (query.isError) return <Callout tone="down">Unable to load general settings.</Callout>;
  const fromEnvironment = data?.public_url_source === "environment";
  return (
    <div className="flex flex-col gap-3.5">
      <FieldPanel title="Instance" aside="Shown in emails and status pages">
        <div className="grid gap-4">
          <Labeled label="Instance name">
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </Labeled>
          <Labeled
            label="Public Core URL"
            hint={
              fromEnvironment
                ? "Currently set by the Core host. Saving here overrides it."
                : "Used in emails, install commands and status links."
            }
          >
            <Input value={url} onChange={(e) => setUrl(e.target.value)} />
          </Labeled>
        </div>
      </FieldPanel>
      <FieldPanel title="Time">
        <div className="grid grid-cols-2 gap-4">
          <Labeled label="Timezone">
            <Select
              aria-label="Timezone"
              className="w-full"
              options={options}
              value={zone}
              onValueChange={setZone}
            />
          </Labeled>
          <Labeled label="Time format">
            <Select
              aria-label="Time format"
              className="w-full"
              options={[
                { value: "24", label: "24 hour" },
                { value: "12", label: "12 hour" },
              ]}
              value={format}
              onValueChange={setFormat}
            />
          </Labeled>
        </div>
      </FieldPanel>
      <div className="flex justify-end">
        <Button
          disabled={save.isPending || !name.trim()}
          onClick={() =>
            save.mutate({
              data: {
                instance_name: name.trim(),
                public_url: url.trim(),
                timezone: zone,
                time_format: format,
              },
            })
          }
        >
          Save settings
        </Button>
      </div>
    </div>
  );
};
