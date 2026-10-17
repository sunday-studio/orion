import { useState } from "react";
import { Link } from "react-router-dom";
import { Button } from "@/design-system";

const DISMISS_KEY = "orion.overview.onboarding.dismissed";

export const isFirstRunDismissed = () => {
  try {
    return localStorage.getItem(DISMISS_KEY) === "1";
  } catch {
    return false;
  }
};

/** Empty Overview: one sentence, one button; dismissible and reopened from Settings (D-006). */
export const OverviewFirstRun = ({ onDismiss }: { onDismiss: () => void }) => {
  const [, setTick] = useState(0);
  const dismiss = () => {
    try {
      localStorage.setItem(DISMISS_KEY, "1");
    } catch {
      // Dismissal still applies for this page view.
    }
    setTick((tick) => tick + 1);
    onDismiss();
  };

  return (
    <section
      aria-label="Getting started"
      className="mx-auto flex max-w-[720px] flex-col items-center px-6 pb-16 pt-[112px] text-center"
    >
      <h1 className="text-[32px] font-medium leading-[1.2] tracking-[-0.025em]">
        Nothing is monitored yet
      </h1>
      <p className="mt-3 text-[14.5px] text-mid">
        Add a server and Orion starts watching it within a minute.
      </p>
      <div className="mt-6 flex items-center gap-2.5">
        <Link to="/servers/new">
          <Button>Add your first server</Button>
        </Link>
        <Link to="/monitors?new=1">
          <Button variant="secondary">Create a monitor</Button>
        </Link>
      </div>
      <p className="mt-9 text-[12.5px] text-quiet">
        Then connect an alert channel,{" "}
        <Link className="underline" to="/apps/new">
          connect an app
        </Link>{" "}
        to see its logs, and publish a status page if you want one.{" "}
        <button type="button" className="underline" onClick={dismiss}>
          Dismiss this
        </button>
      </p>
    </section>
  );
};
