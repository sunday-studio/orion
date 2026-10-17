import type { ColumnDef } from "@tanstack/react-table";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  Button,
  DataTable,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  PageHeader,
} from "@/design-system";
import { type ApiApp, getListAppsQueryKey, useCreateApp, useListApps } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import { AppDetailDrawer } from "./components/app-detail-drawer";

const columns: ColumnDef<ApiApp>[] = [
  { accessorKey: "name", header: "App", cell: ({ row }) => row.original.name ?? row.original.id },
  {
    accessorKey: "environment",
    header: "Environment",
    cell: ({ row }) => row.original.environment || "-",
  },
  { accessorKey: "key_count", header: "Keys", cell: ({ row }) => row.original.key_count ?? 0 },
  {
    accessorKey: "last_ingest_at",
    header: "Last log",
    cell: ({ row }) =>
      row.original.last_ingest_at
        ? formatDate(row.original.last_ingest_at, DATE_TIME_FORMAT)
        : "never",
  },
  {
    accessorKey: "retention_days",
    header: "Retention",
    cell: ({ row }) => `${row.original.retention_days ?? 14} days`,
  },
];

export const AppsPage = () => {
  const queryClient = useQueryClient();
  const appsQuery = useListApps();
  const [selectedId, setSelectedId] = useState<string>();
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("production");
  const createApp = useCreateApp({
    mutation: {
      onSuccess: (response) => {
        void queryClient.invalidateQueries({ queryKey: getListAppsQueryKey() });
        setCreating(false);
        setName("");
        setSelectedId(response.app?.id);
      },
    },
  });
  const apps = appsQuery.data?.apps ?? [];
  const selected = apps.find((app) => app.id === selectedId);

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <PageHeader
          title="Apps"
          description="Applications that send logs to Orion. Each app has its own ingest keys."
        />
        <Button onClick={() => setCreating(true)}>New app</Button>
      </div>

      <DataTable
        columns={columns}
        data={apps}
        getRowId={(app, index) => app.id ?? `app-${index}`}
        isLoading={appsQuery.isLoading}
        error={
          appsQuery.isError
            ? { message: "Unable to load apps", onRetry: () => void appsQuery.refetch() }
            : undefined
        }
        emptyMessage="No apps yet. Create one to start sending logs."
        onRowClick={(app) => setSelectedId(app.id)}
      />

      <Dialog open={creating} onOpenChange={setCreating}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>New app</DialogTitle>
          </DialogHeader>
          <form
            className="grid gap-3"
            onSubmit={(event) => {
              event.preventDefault();
              if (!name.trim()) return;
              createApp.mutate({
                data: { name: name.trim(), environment: environment.trim() || undefined },
              });
            }}
          >
            <label className="grid gap-1 text-[13px]">
              Name
              <Input value={name} onChange={(event) => setName(event.target.value)} required />
            </label>
            <label className="grid gap-1 text-[13px]">
              Environment
              <Input value={environment} onChange={(event) => setEnvironment(event.target.value)} />
            </label>
            {createApp.isError && <p className="text-xs text-destructive">Unable to create app.</p>}
            <DialogFooter>
              <Button type="submit" disabled={!name.trim() || createApp.isPending}>
                Create app
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AppDetailDrawer
        app={selected}
        onOpenChange={(open) => {
          if (!open) setSelectedId(undefined);
        }}
      />
    </div>
  );
};
