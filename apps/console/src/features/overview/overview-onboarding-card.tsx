import { useState } from "react";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";

const DISMISS_KEY = "orion.overview.onboarding.dismissed";

const readDismissed = () => {
  try {
    return localStorage.getItem(DISMISS_KEY) === "1";
  } catch {
    return false;
  }
};

const steps = [
  { label: "Connect your first server", to: "/servers" },
  { label: "Create your first monitor", to: "/monitors" },
  { label: "Add an alert channel", to: "/alerts" },
  { label: "Publish a status page", to: "/status-pages" },
];

export const OverviewOnboardingCard = () => {
  const [dismissed, setDismissed] = useState(readDismissed);
  if (dismissed) return null;

  const dismiss = () => {
    try {
      localStorage.setItem(DISMISS_KEY, "1");
    } catch {
      // Dismissal still applies for this page view.
    }
    setDismissed(true);
  };

  return (
    <section aria-label="Getting started" className="space-y-3 bg-neutral-100 p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-medium">Getting started</h2>
          <p className="text-sm text-neutral-600">Set up Orion in four steps.</p>
        </div>
        <Button type="button" variant="ghost" size="sm" onClick={dismiss}>
          Dismiss
        </Button>
      </div>
      <ol className="list-decimal space-y-1 pl-5 text-sm">
        {steps.map((step) => (
          <li key={step.to}>
            <Link className="underline" to={step.to}>
              {step.label}
            </Link>
          </li>
        ))}
      </ol>
    </section>
  );
};
