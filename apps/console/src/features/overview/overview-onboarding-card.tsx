import { useState } from "react";
import { Link } from "react-router-dom";
import { Button, Card, CardContent } from "@/design-system";

const DISMISS_KEY = "orion.overview.onboarding.dismissed";

const readDismissed = () => {
  try {
    return localStorage.getItem(DISMISS_KEY) === "1";
  } catch {
    return false;
  }
};

const steps = [
  { label: "Add your first server", to: "/servers" },
  { label: "Connect your first app", to: "/settings/apps" },
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
    <Card aria-label="Getting started">
      <CardContent className="space-y-3">
        <div className="flex items-start justify-between gap-3">
          <div>
            <h2 className="text-[15px] font-medium">Getting started</h2>
            <p className="text-[13px] text-quiet">Set up Orion in five steps.</p>
          </div>
          <Button type="button" variant="ghost" size="sm" onClick={dismiss}>
            Dismiss
          </Button>
        </div>
        <ol className="list-decimal space-y-1 pl-5 text-[13px]">
          {steps.map((step) => (
            <li key={step.to}>
              <Link className="text-brand hover:underline" to={step.to}>
                {step.label}
              </Link>
            </li>
          ))}
        </ol>
      </CardContent>
    </Card>
  );
};
