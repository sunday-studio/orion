import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/page-header";
import { CoreMonitorDialog } from "@/features/monitors/components/core-monitor-dialog";
import { MonitorList } from "@/features/monitors/components/monitor-list";
import { type ServiceCoreManagedMonitorCreateRequest, useCreateCoreMonitor } from "@/orion-sdk";
import { useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useState } from "react";

const mutationErrorMessage = (error: unknown, fallback: string) => {
  if (!error) return "";
  if (error instanceof Error) return error.message;
  if (typeof error === "object" && "message" in error) {
    return String((error as { message?: unknown }).message ?? fallback);
  }
  return fallback;
};

export const MonitorsPage = () => {
  const [createOpen, setCreateOpen] = useState(false);
  const queryClient = useQueryClient();
  const refreshMonitors = () => {
    void queryClient.invalidateQueries({ queryKey: ["/v1/monitors"] });
    void queryClient.invalidateQueries({ queryKey: ["/v1/monitors/summary"] });
  };
  const createMonitor = useCreateCoreMonitor({
    mutation: {
      onSuccess: () => {
        refreshMonitors();
        setCreateOpen(false);
      },
    },
  });

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <PageHeader title="Monitors" description="Registered checks across all agents and Core." />
        <Button onClick={() => setCreateOpen(true)}>
          <Plus />
          Core monitor
        </Button>
      </div>
      <MonitorList />
      <CoreMonitorDialog
        error={mutationErrorMessage(createMonitor.error, "Unable to create Core monitor.")}
        isSubmitting={createMonitor.isPending}
        mode="create"
        onOpenChange={setCreateOpen}
        onSubmit={(data) =>
          createMonitor.mutate({ data: data as ServiceCoreManagedMonitorCreateRequest })
        }
        open={createOpen}
      />
    </div>
  );
};
