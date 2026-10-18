import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useToast } from "@/design-system";
import {
  getGetMonitorAvailabilityQueryKey,
  getGetMonitorHistoryQueryKey,
  getGetMonitorQueryKey,
  getGetMonitorSummaryQueryKey,
  getGetMonitorsQueryKey,
  useCloneCoreMonitor,
  useDeleteCoreMonitor,
  usePauseCoreMonitor,
  useResumeCoreMonitor,
  useTestRunMonitor,
} from "@/orion-sdk";

/** Pause, resume, test, clone and delete for one monitor, with cache refresh and toasts. */
export const useMonitorActions = (monitorId: string, name: string) => {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const toast = useToast();
  const refresh = () => {
    for (const queryKey of [
      getGetMonitorQueryKey(monitorId),
      getGetMonitorHistoryQueryKey(monitorId),
      getGetMonitorAvailabilityQueryKey(monitorId),
      getGetMonitorsQueryKey(),
      getGetMonitorSummaryQueryKey(),
    ]) {
      void queryClient.invalidateQueries({ queryKey });
    }
  };
  const failed = (title: string) => () => toast.add({ title });

  const pause = usePauseCoreMonitor({
    mutation: {
      onSuccess: () => {
        toast.add({ title: `${name} paused` });
        refresh();
      },
      onError: failed("Unable to pause the monitor"),
    },
  });
  const resume = useResumeCoreMonitor({
    mutation: {
      onSuccess: () => {
        toast.add({ title: `${name} resumed` });
        refresh();
      },
      onError: failed("Unable to resume the monitor"),
    },
  });
  const test = useTestRunMonitor();
  const clone = useCloneCoreMonitor({
    mutation: {
      onSuccess: (result) => {
        refresh();
        toast.add({ title: `${name} cloned` });
        if (result.monitor?.id) navigate(`/monitors/${result.monitor.id}/edit`);
      },
      onError: failed("Unable to clone the monitor"),
    },
  });
  const remove = useDeleteCoreMonitor({
    mutation: {
      onSuccess: () => {
        refresh();
        toast.add({ title: `${name} deleted` });
        navigate("/monitors", { replace: true });
      },
    },
  });

  return {
    pause: () => pause.mutate({ id: monitorId }),
    resume: () => resume.mutate({ id: monitorId }),
    test: () => test.mutate({ id: monitorId }),
    clone: () => clone.mutate({ id: monitorId }),
    remove: (keepHistory: boolean) =>
      remove.mutate({ id: monitorId, params: { keep_history: keepHistory } }),
    removeError: remove.isError,
    pending:
      pause.isPending || resume.isPending || test.isPending || clone.isPending || remove.isPending,
    testing: test.isPending,
    testResult: test.data,
    testError: test.isError
      ? (test.error as Error).message || "Unable to run the test."
      : undefined,
    clearTestResult: () => test.reset(),
  };
};
